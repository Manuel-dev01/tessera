// Tessera browser QA sweep. Loads / and /console at desktop (1440) and mobile
// (390) on each target, collects console errors, checks horizontal overflow,
// drives the console flow end-to-end (without confirming the on-chain anchor),
// and screenshots every screen. Emits qa/report.json.
//
// Usage: TX=0x<finalized-base-tx> node sweep.mjs [live|local|both]
import { chromium } from "playwright";
import { writeFileSync, mkdirSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";

const here = dirname(fileURLToPath(import.meta.url));
const SHOTS = join(here, "shots");
mkdirSync(SHOTS, { recursive: true });

const TX = process.env.TX || "";
const which = process.argv[2] || "both";
const ALL_TARGETS = {
  live: "https://tessera-console.vercel.app",
  local: "http://localhost:3000",
};
const targets = which === "both" ? ALL_TARGETS : { [which]: ALL_TARGETS[which] };
const viewports = [
  { name: "desktop", width: 1440, height: 900 },
  { name: "mobile", width: 390, height: 844 },
];

const report = [];

function attachCollectors(page) {
  const errors = [];
  page.on("console", (m) => {
    if (m.type() === "error" || m.type() === "warning") errors.push(`[${m.type()}] ${m.text()}`);
  });
  page.on("pageerror", (e) => errors.push(`[pageerror] ${e.message}`));
  page.on("requestfailed", (r) => {
    const url = r.url();
    // ignore Next.js dev HMR churn and favicon; report real API/asset failures
    if (/hot-update|_next\/static\/webpack|favicon/.test(url)) return;
    const f = r.failure();
    errors.push(`[requestfailed] ${r.method()} ${url} :: ${f ? f.errorText : "?"}`);
  });
  return errors;
}

async function overflow(page) {
  return page.evaluate(() => {
    const de = document.documentElement;
    return { scrollWidth: de.scrollWidth, innerWidth: window.innerWidth, overflow: de.scrollWidth > window.innerWidth + 1 };
  });
}

async function snap(page, target, vp, screen) {
  const file = `${target}-${vp}-${screen}.png`;
  try { await page.screenshot({ path: join(SHOTS, file), fullPage: true }); } catch { /* full-page can fail on huge canvases */ await page.screenshot({ path: join(SHOTS, file) }); }
  return `shots/${file}`;
}

async function record(page, errors, target, vp, screen, url, notes = []) {
  const of = await overflow(page);
  const shot = await snap(page, target, vp, screen);
  const entry = { target, viewport: vp, screen, url, overflow: of, consoleErrors: [...errors], notes, screenshot: shot };
  report.push(entry);
  errors.length = 0; // reset for next screen
  return entry;
}

async function safe(label, fn, notes) {
  try { await fn(); return true; } catch (e) { notes.push(`${label} FAILED: ${e.message}`); return false; }
}

async function sweepConsole(page, errors, target, vp) {
  const base = targets[target];
  const notes = [];
  await page.goto(`${base}/console`, { waitUntil: "networkidle", timeout: 45000 }).catch(() => {});
  await page.waitForTimeout(1500);
  await record(page, errors, target, vp, "console-compose", `${base}/console`, notes);

  if (!TX) { report[report.length - 1].notes.push("no TX provided; skipped flow"); return; }

  // Compose: type the tx hash and blur to trigger auto-fill.
  const flowNotes = [];
  const okType = await safe("type txHash", async () => {
    const input = page.locator('input').first();
    await input.click({ timeout: 5000 });
    await input.fill(TX);
    await input.blur();
    await page.waitForTimeout(2500); // getTx round-trip
  }, flowNotes);

  await record(page, errors, target, vp, "console-compose-filled", `${base}/console`, flowNotes);

  if (okType) {
    // Run consensus (a div with this text).
    await safe("click run consensus", async () => {
      await page.getByText("run consensus", { exact: false }).first().click({ timeout: 5000 });
    }, flowNotes);
    // Wait (bounded) for consensus to reach a terminal state.
    const terminal = await page.getByText(/VERIFIED ·|RESOLVED ·|STREAM ERROR/i).first()
      .waitFor({ timeout: 45000 }).then(() => true).catch(() => false);
    if (!terminal) flowNotes.push("UNRESOLVED: consensus never reached a terminal state within 45s");
    await page.waitForTimeout(400);
    await record(page, errors, target, vp, "console-consensus", `${base}/console`, [...flowNotes]);

    // Advance to the proof view (requires clicking the now-enabled button).
    let proofArrived = false;
    if (terminal) {
      await safe("click view proof", async () => { await page.getByText("view proof", { exact: false }).first().click({ timeout: 5000 }); }, flowNotes);
      proofArrived = await page.getByText(/signature (verified|invalid)/i).first()
        .waitFor({ timeout: 12000 }).then(() => true).catch(() => false);
      if (!proofArrived) flowNotes.push("UNRESOLVED: proof view did not render signature status after clicking view proof");
    }
    await page.waitForTimeout(400);
    await record(page, errors, target, vp, "console-proof", `${base}/console`, [...flowNotes]);

    if (proofArrived) {
      await safe("copy JSON", async () => { await page.getByText("copy JSON", { exact: false }).first().click({ timeout: 4000 }); }, flowNotes);
      // Go to handoff but DO NOT confirm (would fire a real on-chain anchor).
      await safe("go to handoff", async () => { await page.getByText("hand off", { exact: false }).first().click({ timeout: 4000 }); await page.waitForTimeout(800); }, flowNotes);
      await record(page, errors, target, vp, "console-handoff", `${base}/console`, [...flowNotes]);
    }
  }

  // Map view.
  await safe("open map", async () => { await page.getByText("proof map", { exact: false }).first().click({ timeout: 4000 }); await page.waitForTimeout(1200); }, flowNotes);
  await record(page, errors, target, vp, "console-map", `${base}/console`, [...flowNotes]);
}

const browser = await chromium.launch();
for (const target of Object.keys(targets)) {
  const base = targets[target];
  for (const vp of viewports) {
    const context = await browser.newContext({ viewport: { width: vp.width, height: vp.height }, deviceScaleFactor: 1 });
    const page = await context.newPage();
    const errors = attachCollectors(page);

    // Landing
    await page.goto(base, { waitUntil: "networkidle", timeout: 45000 }).catch(() => {});
    await page.waitForTimeout(1500);
    await record(page, errors, target, vp.name, "landing", base);

    // Console full flow
    await sweepConsole(page, errors, target, vp.name);

    await context.close();
  }
}
await browser.close();

writeFileSync(join(here, "report.json"), JSON.stringify(report, null, 2));

// Console summary
let issues = 0;
for (const e of report) {
  const ov = e.overflow.overflow ? `OVERFLOW ${e.overflow.scrollWidth}>${e.overflow.innerWidth}` : "";
  const ce = e.consoleErrors.length ? `${e.consoleErrors.length} console` : "";
  const nz = e.notes.filter((n) => /FAILED|UNRESOLVED/.test(n));
  if (ov || ce || nz.length) issues++;
  const flag = ov || ce || nz.length ? "  <== " + [ov, ce, ...nz].filter(Boolean).join(" | ") : "  ok";
  console.log(`${e.target}/${e.viewport}/${e.screen}${flag}`);
}
console.log(`\n${report.length} screens, ${issues} with issues. report -> qa/report.json, shots -> qa/shots/`);
