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
  if (!r.ok) return [];
  return r.json();
}

export async function anchor(proofId: string): Promise<{ anchorTx: string; bond: any }> {
  const r = await fetch(`${API}/api/anchor`, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ ProofID: proofId }),
  });
  const body = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(body.error || "anchor failed");
  return body;
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
  es.addEventListener("source", (e) => h.onSource?.(JSON.parse((e as MessageEvent).data)));
  es.addEventListener("consensus", (e) => h.onConsensus?.(JSON.parse((e as MessageEvent).data)));
  es.addEventListener("proof", (e) => {
    h.onProof?.(JSON.parse((e as MessageEvent).data));
    es.close();
  });
  es.addEventListener("error", (e) => {
    const data = (e as MessageEvent).data;
    if (data) {
      try {
        h.onError?.(JSON.parse(data).error || "stream error");
      } catch {
        h.onError?.("stream error");
      }
    }
    es.close();
  });
  return () => es.close();
}
