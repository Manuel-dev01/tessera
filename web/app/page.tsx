"use client";
import Link from "next/link";
import HeroNet from "@/components/HeroNet";
import { c, mono } from "@/lib/theme";
import { useNarrow } from "@/lib/useNarrow";

const kicker = { fontFamily: mono, fontSize: 12, letterSpacing: ".2em", color: c.accent, marginBottom: 12 } as const;
const h2 = { fontSize: 34, fontWeight: 600, letterSpacing: "-.02em", margin: "0 0 6px", maxWidth: 680 } as const;
const card = { background: c.panel, border: `1px solid ${c.border2}`, borderRadius: 10 } as const;

export default function Landing() {
  const narrow = useNarrow();
  const px = narrow ? 16 : 40;
  const secPad = `${narrow ? 60 : 96}px ${px}px`;
  return (
    <div style={{ display: "flex", minHeight: "100vh" }}>
      {/* left rail */}
      <div style={{ width: 58, flex: "none", borderRight: `1px solid ${c.border}`, background: c.bg2, display: narrow ? "none" : "block" }}>
        <div style={{ position: "sticky", top: 0, height: "100vh", display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "space-between", padding: "20px 0" }}>
          <Logo dots />
          <div style={{ writingMode: "vertical-rl", transform: "rotate(180deg)", fontFamily: mono, fontSize: 10, letterSpacing: ".4em", color: c.faint2 }}>TESSERA · ORACLE · BASE · 8453</div>
          <div style={{ width: 1, flex: 1, margin: "22px 0", background: `repeating-linear-gradient(${c.border} 0 4px,transparent 4px 12px)` }} />
          <div style={{ fontFamily: mono, fontSize: 9, color: c.faint2, writingMode: "vertical-rl", transform: "rotate(180deg)" }}>v1</div>
        </div>
      </div>

      <div style={{ flex: 1, minWidth: 0 }}>
        {/* topbar */}
        <div style={{ position: "sticky", top: 0, zIndex: 50, display: "flex", alignItems: "center", gap: narrow ? 12 : 26, padding: narrow ? "12px 16px" : "16px 40px", background: "rgba(11,14,12,.82)", backdropFilter: "blur(10px)", borderBottom: `1px solid ${c.border}` }}>
          <div style={{ fontFamily: mono, fontWeight: 800, fontSize: narrow ? 15 : 17, letterSpacing: ".28em", paddingLeft: ".28em" }}>TESSERA</div>
          <div style={{ display: narrow ? "none" : "flex", gap: 20, fontFamily: mono, fontSize: 12.5, color: c.mute, marginLeft: 8 }}>
            <a href="#protocol" style={{ color: c.mute }}>--protocol</a>
            <a href="#proof" style={{ color: c.mute }}>--proof</a>
            <a href="#consensus" style={{ color: c.mute }}>--consensus</a>
            <a href="#callable" style={{ color: c.mute }}>--pricing</a>
            <Link href="/console" style={{ color: c.mute }}>--console</Link>
          </div>
          <div style={{ marginLeft: "auto", display: "flex", alignItems: "center", gap: 16 }}>
            <span style={{ display: narrow ? "none" : "flex", alignItems: "center", gap: 7, fontFamily: mono, fontSize: 11.5, color: c.mute }}>
              <span style={{ width: 7, height: 7, borderRadius: "50%", background: c.accent, boxShadow: `0 0 7px ${c.accent}`, animation: "tsPulse 2s ease-in-out infinite" }} />ONLINE · base
            </span>
            <Link href="/console" style={{ fontFamily: mono, fontSize: 12, color: c.bg, background: c.accent, padding: "8px 15px", borderRadius: 6, fontWeight: 600 }}>$ open console</Link>
          </div>
        </div>

        {/* hero */}
        <section style={{ position: "relative", overflow: "hidden", padding: narrow ? "44px 16px 60px" : "78px 40px 96px", minHeight: 600, borderBottom: `1px solid ${c.border}` }}>
          <HeroNet style={{ position: "absolute", top: -30, right: -120, width: 760, height: 700, pointerEvents: "none", opacity: 0.95 }} />
          <div style={{ position: "absolute", top: -30, right: -120, width: 760, height: 700, background: "radial-gradient(circle at 62% 46%,rgba(182,255,61,.06),transparent 55%)", pointerEvents: "none" }} />
          <div style={{ position: "relative", maxWidth: 680 }}>
            <div style={{ fontFamily: mono, fontSize: 12, letterSpacing: ".24em", color: c.mute, marginBottom: 26 }}>AGENTIC VERIFICATION ORACLE · CAP PROVIDER · BASE</div>
            <h1 style={{ fontSize: narrow ? 38 : 64, lineHeight: 1.05, fontWeight: 700, letterSpacing: "-.028em", margin: 0 }}>
              Turn an on-chain event into a <span style={{ color: c.accent }}>signed proof</span> another agent can trust.
            </h1>
            <p style={{ fontSize: 18, lineHeight: 1.55, color: c.dim, maxWidth: 560, margin: "26px 0 0" }}>
              Tessera is a paid, callable CAP provider on Base. Agents hire it to verify that a specific event occurred — it reaches consensus and returns a cryptographically signed <b style={{ color: c.text }}>AgenticVerificationProof</b>, one proof-half ready to hand off as trustless evidence.
            </p>
            <div style={{ marginTop: 34, display: "inline-flex", flexWrap: "wrap", alignItems: "center", maxWidth: "100%", background: c.panel, border: `1px solid ${c.border2}`, borderRadius: 9, padding: narrow ? "12px 14px" : "14px 18px", fontFamily: mono, fontSize: narrow ? 12 : 14.5, boxShadow: "0 8px 30px rgba(0,0,0,.4)" }}>
              <span style={{ color: c.accent }}>agent@base</span><span style={{ color: c.mute }}>:~$</span>&nbsp;<span style={{ color: c.text }}>tessera.verify(&#123; event, tx &#125;)</span>&nbsp;<span style={{ color: c.mute }}>→</span>&nbsp;<span style={{ color: c.accent }}>AVP‹v1›</span>
              <span style={{ display: "inline-block", width: 9, height: 17, background: c.accent, marginLeft: 5, animation: "tsBlink 1.1s step-end infinite" }} />
            </div>
            <div style={{ display: "flex", flexWrap: "wrap", gap: narrow ? 18 : 34, marginTop: 38, fontFamily: mono }}>
              <Readout v="11/11" label="sources agreed" />
              <div style={{ width: 1, background: c.border }} />
              <Readout v="~4s" label="time to proof" />
              <div style={{ width: 1, background: c.border }} />
              <Readout v="0.005 USDC" label="per call" accent />
            </div>
          </div>
        </section>

        {/* 01 protocol */}
        <section id="protocol" style={{ padding: secPad, borderBottom: `1px solid ${c.border}` }}>
          <div style={kicker}>// 01 — PROTOCOL</div>
          <h2 style={h2}>A proof-half travels between two agents.</h2>
          <p style={{ color: c.dim, maxWidth: 600, margin: "0 0 60px", fontSize: 16, lineHeight: 1.55 }}>
            One agent calls. Tessera runs consensus across independent verifiers. The signed AVP half is returned to the caller — evidence a counterparty can independently validate.
          </p>
          <div style={{ position: "relative", display: "grid", gridTemplateColumns: narrow ? "1fr" : "1fr 1.4fr 1fr", gap: narrow ? 16 : 0, alignItems: "center", maxWidth: 1000 }}>
            <div style={{ display: narrow ? "none" : "block", position: "absolute", left: "12%", right: "12%", top: "50%", height: 2, background: `repeating-linear-gradient(90deg,${c.border2} 0 8px,transparent 8px 16px)`, backgroundSize: "16px 2px", animation: "tsDash 1.4s linear infinite" }} />
            <AgentCard role="CALLER" name="Agent A" note={<>needs proof that<br />an event occurred</>} align="start" narrow={narrow} />
            <div style={{ justifySelf: "center", background: "linear-gradient(#12161a,#0e1210)", border: `1px solid ${c.accentBorder}`, borderRadius: 12, padding: "22px 26px", textAlign: "center", boxShadow: "0 0 40px rgba(182,255,61,.08)", position: "relative", zIndex: 2 }}>
              <div style={{ margin: "0 auto 10px", width: "fit-content" }}><Logo big /></div>
              <div style={{ fontFamily: mono, fontWeight: 700, letterSpacing: ".18em", fontSize: 13 }}>TESSERA</div>
              <div style={{ fontSize: 12, color: c.accent, fontFamily: mono, marginTop: 6 }}>consensus · 11/11</div>
              <div style={{ fontSize: 11.5, color: c.mute, marginTop: 2 }}>signs AVP‹v1›</div>
            </div>
            <AgentCard role="COUNTERPARTY" name="Agent B" note={<>validates the<br />signed half</>} align="end" narrow={narrow} />
          </div>
        </section>

        {/* 02 proof */}
        <section id="proof" style={{ padding: secPad, borderBottom: `1px solid ${c.border}`, background: c.bg2 }}>
          <div style={kicker}>// 02 — THE PROOF</div>
          <h2 style={{ ...h2, margin: "0 0 50px" }}>One object. Signed, consensus-backed, machine-readable.</h2>
          <div style={{ display: "grid", gridTemplateColumns: narrow ? "1fr" : "minmax(0,620px) 340px", gap: narrow ? 22 : 44, alignItems: "start", maxWidth: 1060 }}>
            <div style={{ ...card, overflow: "hidden", position: "relative" }}>
              <div style={{ position: "absolute", left: 0, right: 0, height: 44, background: "linear-gradient(#b6ff3d,transparent)", opacity: 0.04, animation: "tsScan 6s linear infinite", pointerEvents: "none" }} />
              <div style={{ display: "flex", alignItems: "center", gap: 9, padding: "13px 16px", borderBottom: `1px solid ${c.border2}`, fontFamily: mono, fontSize: 11.5, color: c.mute }}>
                <span style={{ width: 9, height: 9, borderRadius: "50%", background: c.accent, boxShadow: `0 0 8px ${c.accent}`, animation: "tsPulse 2s ease-in-out infinite" }} />
                AgenticVerificationProof · <span style={{ color: c.text }}>v1</span>
                <span style={{ marginLeft: "auto", color: c.accent, border: `1px solid ${c.accent}`, borderRadius: 5, padding: "2px 9px", fontSize: 10, letterSpacing: ".1em" }}>VERIFIED</span>
              </div>
              <pre style={{ margin: 0, padding: "22px 24px", fontFamily: mono, fontSize: 13, lineHeight: 1.85, color: c.mute, whiteSpace: "pre-wrap" }}>
{`{`}
{`  `}<K>schemaVersion</K>: <V>"avp/1.0"</V>,{`
  `}<K>verified</K>: <V>true</V>, <K>chainId</K>: <V>8453</V>,{`
  `}<K>blockNumber</K>: <V>48494146</V>,{`
  `}<K>txHash</K>: <V>"0x13cf…5105"</V>,{`
  `}<K>consensus</K>: <V>{`{ agreed: 11, quorum: 9 }`}</V>,{`
  `}<K>finality</K>: <V>{`{ finalized: true }`}</V>,{`
  `}<K>attestation</K>: <span style={{ color: c.text }}>{`{ scheme: "EIP-191", … }`}</span>{`
}`}
              </pre>
            </div>
            <div style={{ display: "flex", flexDirection: "column", gap: 20, paddingTop: 8 }}>
              <Annot k="consensus" d="How many of the 11 independent sources agreed on the block and status." />
              <Divider />
              <Annot k="finality" d="The block is under Base's finalized tag — no reorg can revert it." />
              <Divider />
              <Annot k="attestation" d="EIP-191 signature over the canonical proof; verify it with the signer's address alone." />
              <Divider />
              <Annot k="bond" d="An on-chain USDC stake anyone can slash if the proof is fraudulent." />
            </div>
          </div>
        </section>

        {/* 03 consensus */}
        <section id="consensus" style={{ padding: secPad, borderBottom: `1px solid ${c.border}` }}>
          <div style={kicker}>// 03 — CONSENSUS</div>
          <h2 style={h2}>No single node is trusted. The threshold is.</h2>
          <p style={{ color: c.dim, maxWidth: 600, margin: "0 0 46px", fontSize: 16, lineHeight: 1.55 }}>
            Eleven independent sources observe the chain. A proof is only signed when a supermajority agrees on blockHash + status — a dynamic quorum of the live responders.
          </p>
          <div style={{ display: "flex", gap: 9, flexWrap: "wrap", maxWidth: 900, marginBottom: 34 }}>
            {["base.org", "publicnode", "drpc", "1rpc", "blastapi", "tenderly", "nodies", "lava", "bloxroute", "zan", "blockscout"].map((n, i) => (
              <div key={n} style={{ display: "flex", alignItems: "center", gap: 8, background: i < 9 ? c.panel : c.panel2, border: `1px solid ${i < 9 ? c.accentBorder : c.border3}`, borderRadius: 7, padding: "9px 13px", fontFamily: mono, fontSize: 12, color: i < 9 ? c.text : c.mute }}>
                <span style={{ width: 7, height: 7, borderRadius: "50%", background: i < 9 ? c.accent : c.faint2, boxShadow: i < 9 ? `0 0 6px ${c.accent}` : "none" }} />{n} <span style={{ color: c.mute }}>{i < 9 ? "✓" : "·"}</span>
              </div>
            ))}
          </div>
          <div style={{ maxWidth: 560 }}>
            <div style={{ display: "flex", justifyContent: "space-between", fontFamily: mono, fontSize: 12, color: c.mute, marginBottom: 8 }}><span>QUORUM THRESHOLD</span><span>quorum <b style={{ color: c.accent }}>9</b> / 11 · min 7</span></div>
            <div style={{ height: 10, background: c.panel, border: `1px solid ${c.border2}`, borderRadius: 6, overflow: "hidden", display: "flex" }}>
              <div style={{ width: "81.8%", background: `linear-gradient(90deg,${c.accentDim},${c.accent})` }} />
            </div>
            <div style={{ fontFamily: mono, fontSize: 11, color: c.mute, marginTop: 8 }}>threshold met — proof resolved &amp; signed</div>
          </div>
        </section>

        {/* 04 callable */}
        <section id="callable" style={{ padding: secPad, borderBottom: `1px solid ${c.border}`, background: c.bg2 }}>
          <div style={kicker}>// 04 — CALLABLE</div>
          <h2 style={h2}>Metered. Pay per verification.</h2>
          <p style={{ color: c.dim, maxWidth: 580, margin: "0 0 48px", fontSize: 16, lineHeight: 1.55 }}>Install the SDK, point it at Base, and call. You're billed only for proofs that resolve.</p>
          <div style={{ display: "grid", gridTemplateColumns: narrow ? "1fr" : "minmax(0,1fr) 320px", gap: narrow ? 18 : 32, alignItems: "stretch", maxWidth: 1060 }}>
            <div style={{ ...card, overflow: "hidden" }}>
              <div style={{ display: "flex", alignItems: "center", gap: 7, padding: "12px 16px", borderBottom: `1px solid ${c.border2}` }}>
                {[0, 1, 2].map((i) => <span key={i} style={{ width: 11, height: 11, borderRadius: "50%", background: c.border2 }} />)}
                <span style={{ marginLeft: 8, fontFamily: mono, fontSize: 11.5, color: c.mute }}>verify.ts</span>
              </div>
              <pre style={{ margin: 0, padding: "22px 24px", fontFamily: mono, fontSize: 13.5, lineHeight: 1.9, color: c.text, overflowX: "auto" }}>
{`import { Tessera } from "@tessera/cap";

const t = new Tessera({ network: "base" });

const proof = await t.verify({
  txHash: "0x13cf…5105",
});

// → AgenticVerificationProof‹v1› · VERIFIED
proof.attestation.signature; // 0xdaea…
proof.consensus;             // { agreed: 11, quorum: 9 }`}
              </pre>
            </div>
            <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
              <div style={{ background: c.panel, border: `1px solid ${c.accentBorder}`, borderRadius: 10, padding: 24 }}>
                <div style={{ fontFamily: mono, fontSize: 11, color: c.mute, letterSpacing: ".12em" }}>PER RESOLVED PROOF</div>
                <div style={{ fontFamily: mono, fontSize: 38, fontWeight: 700, color: c.accent, marginTop: 8 }}>0.005<span style={{ fontSize: 16, color: c.mute }}> USDC</span></div>
                <div style={{ fontSize: 13, color: c.dim, marginTop: 6, lineHeight: 1.5 }}>no charge for calls that fail to reach threshold.</div>
              </div>
              <div style={{ background: c.panel2, border: `1px solid ${c.border3}`, borderRadius: 10, padding: 20, fontSize: 13, color: c.dim, lineHeight: 1.7 }}>
                <Row a="on-chain bond" b="USDC · slashable" /><Divider />
                <Row a="finality gate" b="finalized tag" /><Divider />
                <Row a="settlement" b="on-chain, Base" />
              </div>
            </div>
          </div>
        </section>

        {/* CTA */}
        <section style={{ padding: narrow ? "70px 16px" : "110px 40px", textAlign: "center", position: "relative", overflow: "hidden" }}>
          <div style={{ position: "absolute", inset: 0, background: "radial-gradient(circle at 50% 30%,rgba(182,255,61,.07),transparent 60%)", pointerEvents: "none" }} />
          <div style={{ position: "relative" }}>
            <h2 style={{ fontSize: narrow ? 32 : 48, fontWeight: 700, letterSpacing: "-.028em", margin: "0 0 24px" }}>Hire the oracle.</h2>
            <Link href="/console" style={{ display: "inline-block", color: c.bg, background: c.accent, padding: "14px 28px", borderRadius: 8, fontFamily: mono, fontWeight: 600, fontSize: 14 }}>$ open console →</Link>
          </div>
        </section>

        {/* footer — implementation ends here */}
        <footer style={{ borderTop: `1px solid ${c.border}`, padding: narrow ? "28px 16px" : "34px 40px", display: "flex", alignItems: "center", gap: 20, flexWrap: "wrap", fontFamily: mono, fontSize: 12, color: c.mute }}>
          <span style={{ fontWeight: 800, letterSpacing: ".24em", color: c.text }}>TESSERA</span>
          <span style={{ color: c.faint2 }}>·</span><span>agentic verification oracle</span>
          <span style={{ marginLeft: "auto", display: "flex", gap: 20 }}>
            <a href="#protocol" style={{ color: c.mute }}>protocol</a>
            <a href="#proof" style={{ color: c.mute }}>avp spec</a>
            <Link href="/console" style={{ color: c.mute }}>console</Link>
            <span>© 2026</span>
          </span>
        </footer>
      </div>
    </div>
  );
}

function Logo({ dots, big }: { dots?: boolean; big?: boolean }) {
  const s = big ? 12 : 9;
  return (
    <div style={{ display: "grid", gridTemplateColumns: `${s}px ${s}px`, gridTemplateRows: `${s}px ${s}px`, gap: 2 }}>
      <div style={{ background: c.accent }} /><div style={{ border: `1px solid ${c.border2}` }} />
      <div style={{ border: `1px solid ${c.border2}` }} /><div style={{ background: c.text }} />
    </div>
  );
}
function Readout({ v, label, accent }: { v: string; label: string; accent?: boolean }) {
  return (
    <div>
      <div style={{ fontSize: 26, fontWeight: 700, color: accent ? c.accent : c.text }}>{v}</div>
      <div style={{ fontSize: 11, color: c.mute, marginTop: 3, letterSpacing: ".06em" }}>{label}</div>
    </div>
  );
}
function AgentCard({ role, name, note, align, narrow }: { role: string; name: string; note: React.ReactNode; align: "start" | "end"; narrow?: boolean }) {
  return (
    <div style={{ justifySelf: narrow ? "stretch" : align, background: c.panel, border: `1px solid ${c.border2}`, borderRadius: 10, padding: "20px 22px", width: narrow ? "100%" : 230, position: "relative", zIndex: 2 }}>
      <div style={{ fontFamily: mono, fontSize: 11, color: c.mute }}>{role}</div>
      <div style={{ fontSize: 19, fontWeight: 600, marginTop: 4 }}>{name}</div>
      <div style={{ fontFamily: mono, fontSize: 11.5, color: c.mute, marginTop: 8 }}>{note}</div>
    </div>
  );
}
function Annot({ k, d }: { k: string; d: string }) {
  return (
    <div>
      <div style={{ fontFamily: mono, fontSize: 11, color: c.accent }}>{k}</div>
      <div style={{ fontSize: 13.5, color: c.dim, marginTop: 4, lineHeight: 1.5 }}>{d}</div>
    </div>
  );
}
function Divider() { return <div style={{ height: 1, background: c.border }} />; }
function Row({ a, b }: { a: string; b: string }) {
  return <div style={{ display: "flex", justifyContent: "space-between" }}><span style={{ color: c.mute }}>{a}</span><span>{b}</span></div>;
}
function K({ children }: { children: React.ReactNode }) { return <span style={{ color: c.mute }}>{children}</span>; }
function V({ children }: { children: React.ReactNode }) { return <span style={{ color: c.accent }}>{children}</span>; }
