import type { Health, TxPreview, SourceReport, ConsensusSummary, Proof, StoredProof } from "./types";

export const API = process.env.NEXT_PUBLIC_API_BASE || "http://localhost:8787";

export async function getHealth(): Promise<Health> {
  const r = await fetch(`${API}/api/health`);
  if (!r.ok) throw new Error("health failed");
  return r.json();
}

export async function getTx(hash: string): Promise<TxPreview> {
  const r = await fetch(`${API}/api/tx?hash=${encodeURIComponent(hash)}`);
  if (!r.ok) throw new Error((await r.json().catch(() => ({}))).error || "tx lookup failed");
  return r.json();
}

export async function getProofs(): Promise<StoredProof[]> {
  const r = await fetch(`${API}/api/proofs`);
  if (!r.ok) throw new Error(`proofs failed (${r.status})`);
  return r.json();
}

// anchor commits the proof on-chain. It bounds the request with a client-side
// timeout (the server caps its own work at 90s) so a dropped socket can't leave
// the handoff button spinning forever. Returns the re-signed proof so the UI can
// replace its copy (the signature covers the now-anchored bond fields).
export async function anchor(proofId: string): Promise<{ anchorTx: string; bond: any; proof?: Proof }> {
  const ctrl = new AbortController();
  const timer = setTimeout(() => ctrl.abort(), 100_000);
  try {
    const r = await fetch(`${API}/api/anchor`, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ ProofID: proofId }),
      signal: ctrl.signal,
    });
    const body = await r.json().catch(() => ({}));
    if (!r.ok) throw new Error(body.error || "anchor failed");
    return body;
  } catch (e: any) {
    if (e?.name === "AbortError") throw new Error("anchor timed out — check Basescan for the tx");
    throw e;
  } finally {
    clearTimeout(timer);
  }
}

export interface StreamHandlers {
  onSource?: (r: SourceReport) => void;
  onConsensus?: (c: ConsensusSummary) => void;
  onProof?: (p: Proof) => void;
  onError?: (msg: string) => void;
}

// verifyStream opens the SSE and dispatches events. Returns a closer.
export function verifyStream(
  params: { txHash: string; address?: string; blockNumber?: number; event?: string; logIndex?: number; claim?: string },
  h: StreamHandlers
): () => void {
  const q = new URLSearchParams();
  q.set("txHash", params.txHash);
  if (params.address) q.set("address", params.address);
  if (params.blockNumber) q.set("blockNumber", String(params.blockNumber));
  if (params.event) q.set("event", params.event);
  if (params.logIndex != null) q.set("logIndex", String(params.logIndex));
  if (params.claim) q.set("claim", params.claim);

  const es = new EventSource(`${API}/api/verify/stream?${q.toString()}`);

  // Watchdog: never let the UI wait forever. If no proof (or structured error)
  // arrives within the budget, surface a timeout. Reset on each event so a live
  // but slow stream isn't cut off mid-flight.
  let settled = false;
  let watchdog: ReturnType<typeof setTimeout>;
  const finish = () => {
    if (settled) return;
    settled = true;
    clearTimeout(watchdog);
    es.close();
  };
  const arm = (ms: number, msg: string) => {
    clearTimeout(watchdog);
    watchdog = setTimeout(() => {
      if (settled) return;
      h.onError?.(msg);
      finish();
    }, ms);
  };
  arm(20_000, "no source responded — the oracle may be unreachable");

  es.addEventListener("source", (e) => {
    if (settled) return;
    arm(45_000, "verification stalled — no proof after sources responded");
    h.onSource?.(JSON.parse((e as MessageEvent).data));
  });
  es.addEventListener("consensus", (e) => {
    if (settled) return;
    arm(45_000, "verification stalled — consensus reached but no signed proof");
    h.onConsensus?.(JSON.parse((e as MessageEvent).data));
  });
  es.addEventListener("proof", (e) => {
    if (settled) return;
    h.onProof?.(JSON.parse((e as MessageEvent).data));
    finish();
  });
  es.addEventListener("error", (e) => {
    if (settled) return;
    const data = (e as MessageEvent).data;
    // Server-emitted errors carry a JSON payload; native transport errors (server
    // down, dropped connection, CORS/TLS) are dataless — surface both so the UI
    // never silently stalls.
    let msg = "connection to the oracle was lost";
    if (data) {
      try { msg = JSON.parse(data).error || "stream error"; } catch { msg = "stream error"; }
    }
    h.onError?.(msg);
    finish();
  });
  return finish;
}
