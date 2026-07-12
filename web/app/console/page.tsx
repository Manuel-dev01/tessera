"use client";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import Link from "next/link";
import VerifierSphere from "@/components/VerifierSphere";
import ProofGraph from "@/components/ProofGraph";
import { c, mono } from "@/lib/theme";
import * as api from "@/lib/api";
import { verifyProofSignature } from "@/lib/avp";
import type { Health, TxPreview, SourceReport, ConsensusSummary, Proof, StoredProof } from "@/lib/types";

type Stage = "compose" | "consensus" | "proof" | "handoff";
const ORDER: Stage[] = ["compose", "consensus", "proof", "handoff"];

export default function Console() {
  const [health, setHealth] = useState<Health | null>(null);
  const [view, setView] = useState<"pipeline" | "map">("pipeline");
  const [stage, setStage] = useState<Stage>("compose");

  // compose
  const [txHash, setTxHash] = useState("");
  const [preview, setPreview] = useState<TxPreview | null>(null);
  const [previewErr, setPreviewErr] = useState("");
  const [claim, setClaim] = useState("");
  const [eventSig, setEventSig] = useState("");

  // consensus (live)
  const [reports, setReports] = useState<Record<string, SourceReport>>({});
  const [summary, setSummary] = useState<ConsensusSummary | null>(null);
  const [proof, setProof] = useState<Proof | null>(null);
  const [streamErr, setStreamErr] = useState("");
  const [t0, setT0] = useState(0);
  const [elapsed, setElapsed] = useState(0);
  const closer = useRef<(() => void) | undefined>(undefined);

  // handoff
  const [counterparty, setCounterparty] = useState("did:agent:0xB…settlement");
  const [anchorState, setAnchorState] = useState<"idle" | "anchoring" | "done" | "error">("idle");
  const [anchorTx, setAnchorTx] = useState("");
  const [anchorErr, setAnchorErr] = useState("");

  // map
  const [proofs, setProofs] = useState<StoredProof[]>([]);
  const [selectedId, setSelectedId] = useState("");

  useEffect(() => { api.getHealth().then(setHealth).catch(() => {}); }, []);
  useEffect(() => {
    if (stage !== "consensus" || proof || streamErr) return;
    const iv = setInterval(() => setElapsed((performance.now() - t0) / 1000), 100);
    return () => clearInterval(iv);
  }, [stage, proof, streamErr, t0]);
  useEffect(() => () => closer.current?.(), []);

  const sources = health?.sources ?? [];
  const lit = useMemo(() => sources.map((n) => !!reports[n]?.ok && !!reports[n]?.found), [sources, reports]);
  const foundCount = lit.filter(Boolean).length;

  const sigCheck = useMemo(() => (proof ? verifyProofSignature(proof) : null), [proof]);

  async function onTxBlur() {
    setPreviewErr(""); setPreview(null);
    const h = txHash.trim();
    if (!/^0x[0-9a-fA-F]{64}$/.test(h)) { if (h) setPreviewErr("not a 32-byte tx hash"); return; }
    try {
      const p = await api.getTx(h);
      setPreview(p);
      if (!claim) setClaim(`tx ${h.slice(0, 10)}… on Base`);
    } catch (e: any) { setPreviewErr(e.message || "lookup failed"); }
  }

  function runConsensus() {
    if (!/^0x[0-9a-fA-F]{64}$/.test(txHash.trim())) { setPreviewErr("enter a valid tx hash first"); return; }
    setReports({}); setSummary(null); setProof(null); setStreamErr(""); setElapsed(0);
    setT0(performance.now());
    setStage("consensus");
    closer.current?.();
    closer.current = api.verifyStream(
      { txHash: txHash.trim(), address: preview?.from, blockNumber: preview?.blockNumber, event: eventSig || undefined, claim },
      {
        onSource: (r) => setReports((m) => ({ ...m, [r.name]: r })),
        onConsensus: (s) => setSummary(s),
        onProof: (p) => setProof(p),
        onError: (msg) => setStreamErr(msg),
      }
    );
  }

  async function confirmHandoff() {
    setAnchorErr("");
    if (health?.bondAnchorReady && proof?.bond?.proofId) {
      setAnchorState("anchoring");
      try {
        const res = await api.anchor(proof.bond.proofId);
        setAnchorTx(res.anchorTx);
        setProof((p) => (p && p.bond ? { ...p, bond: { ...p.bond, anchored: true, anchorTx: res.anchorTx } } : p));
        setAnchorState("done");
      } catch (e: any) { setAnchorErr(e.message || "anchor failed"); setAnchorState("error"); }
    } else {
      setAnchorState("done"); // export-only handoff
    }
    api.getProofs().then(setProofs).catch(() => {});
  }

  function openMap() { api.getProofs().then((ps) => { setProofs(ps); if (ps[0]) setSelectedId(ps[0].proofId); }).catch(() => {}); setView("map"); }
  function newVerification() { setView("pipeline"); setStage("compose"); setTxHash(""); setPreview(null); setProof(null); setReports({}); setSummary(null); setAnchorState("idle"); setAnchorTx(""); }

  const proofReady = !!proof && !streamErr;

  return (
    <div style={{ minHeight: "100vh", display: "flex", flexDirection: "column" }}>
      {/* top bar */}
      <div style={{ display: "flex", alignItems: "center", gap: 20, padding: "14px 26px", borderBottom: `1px solid ${c.border}`, background: c.bg2 }}>
        <Link href="/" style={{ display: "flex", alignItems: "center", gap: 11 }}>
          <Logo />
          <span style={{ fontFamily: mono, fontWeight: 800, fontSize: 14, letterSpacing: ".22em", paddingLeft: ".22em", color: c.text }}>TESSERA</span>
        </Link>
        <span style={{ fontFamily: mono, fontSize: 11, color: c.mute }}>console · base · {health?.sourceCount ?? 11} verifiers</span>
        <div style={{ marginLeft: "auto", display: "flex", alignItems: "center", gap: 14, fontFamily: mono, fontSize: 12 }}>
          {view === "pipeline" ? (
            <span onClick={openMap} style={{ cursor: "pointer", color: c.text, background: c.panel, border: `1px solid ${c.border2}`, borderRadius: 7, padding: "7px 13px" }}>◇ proof map · {proofs.length || ""} →</span>
          ) : (
            <span onClick={() => setView("pipeline")} style={{ cursor: "pointer", color: c.text, background: c.panel, border: `1px solid ${c.border2}`, borderRadius: 7, padding: "7px 13px" }}>← pipeline</span>
          )}
          <span style={{ display: "flex", alignItems: "center", gap: 7, color: c.mute }}>
            <span style={{ width: 7, height: 7, borderRadius: "50%", background: c.accent, boxShadow: `0 0 7px ${c.accent}`, animation: "tsPulse 2s ease-in-out infinite" }} />
            {health ? `${health.oracleEth} ETH` : "…"}
          </span>
        </div>
      </div>

      {view === "pipeline" ? (
        <div style={{ flex: 1, display: "flex", flexDirection: "column" }}>
          {/* stage rail */}
          <div style={{ display: "flex", alignItems: "center", padding: "22px 30px", borderBottom: `1px solid ${c.border}`, background: c.bg2, fontFamily: mono, fontSize: 12.5 }}>
            {ORDER.map((st, i) => (
              <RailStep key={st} label={st.toUpperCase()} idx={i} cur={ORDER.indexOf(stage)} last={i === 3}
                onClick={() => { if (i <= ORDER.indexOf(stage)) setStage(st); }} />
            ))}
          </div>

          <div style={{ flex: 1 }}>
            {stage === "compose" && (
              <Compose {...{ txHash, setTxHash, onTxBlur, preview, previewErr, claim, setClaim, eventSig, setEventSig, health, runConsensus }} />
            )}
            {stage === "consensus" && (
              <Consensus {...{ sources, lit, reports, summary, proof, streamErr, elapsed, foundCount, health, proofReady, goCompose: () => setStage("compose"), goProof: () => setStage("proof") }} />
            )}
            {stage === "proof" && proof && (
              <ProofView {...{ proof, sigCheck, claim, goConsensus: () => setStage("consensus"), goHandoff: () => setStage("handoff") }} />
            )}
            {stage === "handoff" && proof && (
              <Handoff {...{ proof, counterparty, setCounterparty, anchorState, anchorTx, anchorErr, health, confirmHandoff, goProof: () => setStage("proof"), newVerification, openMap }} />
            )}
          </div>
        </div>
      ) : (
        <MapView {...{ proofs, selectedId, setSelectedId, newVerification }} />
      )}
    </div>
  );
}

/* ---------- COMPOSE ---------- */
function Compose({ txHash, setTxHash, onTxBlur, preview, previewErr, claim, setClaim, eventSig, setEventSig, health, runConsensus }: any) {
  return (
    <div style={{ maxWidth: 820, margin: "0 auto", padding: "44px 30px", animation: "tsUp .3s ease" }}>
      <div style={{ fontSize: 22, fontWeight: 600, marginBottom: 4 }}>Compose a verification</div>
      <div style={{ color: c.dim2, fontSize: 13.5, marginBottom: 30 }}>Paste a Base transaction hash. Tessera polls its 11 independent sources, checks finality, and signs the resolved proof.</div>
      <div style={{ display: "flex", flexDirection: "column", gap: 20 }}>
        <Field label="TX HASH · base">
          <input value={txHash} onChange={(e) => setTxHash(e.target.value)} onBlur={onTxBlur} placeholder="0x…" spellCheck={false}
            style={{ width: "100%", background: c.panel, border: `1px solid ${previewErr ? c.danger : c.accent}`, borderRadius: 8, padding: "12px 14px", fontSize: 13, color: c.text }} />
          {previewErr && <div style={{ color: c.danger, fontFamily: mono, fontSize: 11, marginTop: 6 }}>{previewErr}</div>}
        </Field>
        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 16 }}>
          <Chip label="CONTRACT (to)" value={preview?.to || "—"} />
          <Chip label="BLOCK" value={preview ? preview.blockNumber.toLocaleString() : "—"} />
        </div>
        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 16 }}>
          <Chip label="SENDER (from)" value={preview?.from || "—"} />
          <Chip label="STATUS" value={preview ? (preview.status === 1 ? "success" : "reverted") : "—"} accent={preview?.status === 1} />
        </div>
        <Field label="EVENT SIGNATURE · optional">
          <input value={eventSig} onChange={(e) => setEventSig(e.target.value)} placeholder="Transfer(address,address,uint256)" spellCheck={false}
            style={{ width: "100%", background: c.panel, border: `1px solid ${c.border2}`, borderRadius: 8, padding: "12px 14px", fontSize: 13, color: c.text }} />
        </Field>
        <Field label="CLAIM LABEL · optional (human context, not verified)">
          <input value={claim} onChange={(e) => setClaim(e.target.value)} placeholder="0x7a3f… received 25000 USDC"
            style={{ width: "100%", background: c.panel, border: `1px solid ${c.border2}`, borderRadius: 8, padding: "12px 14px", fontSize: 13, color: c.text }} />
        </Field>
        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginTop: 6 }}>
          <div style={{ fontFamily: mono, fontSize: 12.5, color: c.mute }}>{health ? `${health.sourceCount} sources online · quorum ${(health.quorumFraction).toFixed(2)} · min ${health.minResponders}` : "…"}</div>
          <div onClick={runConsensus} style={{ cursor: "pointer", fontFamily: mono, fontSize: 13.5, color: c.bg, background: c.accent, padding: "13px 22px", borderRadius: 8, fontWeight: 700 }}>run consensus →</div>
        </div>
      </div>
    </div>
  );
}

/* ---------- CONSENSUS ---------- */
function Consensus({ sources, lit, reports, summary, proof, streamErr, elapsed, foundCount, health, proofReady, goCompose, goProof }: any) {
  const total = sources.length || 11;
  const agreed = summary?.agreed ?? foundCount;
  const quorum = summary?.quorum ?? "…";
  const pct = Math.round((agreed / total) * 100);
  const label = streamErr ? "STREAM ERROR" : proof ? (proof.verified ? `VERIFIED · ${agreed}/${total}` : `RESOLVED · ${agreed}/${total}`) : `FORMING CONSENSUS · ${foundCount}/${total}`;
  return (
    <div style={{ display: "grid", gridTemplateColumns: "420px 1fr", minHeight: 460, animation: "tsUp .3s ease" }}>
      <div style={{ borderRight: `1px solid ${c.border}`, display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", position: "relative", background: "radial-gradient(circle at 50% 45%,rgba(182,255,61,.05),transparent 60%)" }}>
        <VerifierSphere lit={lit} style={{ width: 360, height: 320 }} />
        <div style={{ fontFamily: mono, fontSize: 12.5, color: proof && !proof.verified ? c.warn : c.accent }}>{label}</div>
        <div style={{ fontFamily: mono, fontSize: 11, color: c.mute, marginTop: 4 }}>elapsed {elapsed.toFixed(1)}s · {total} sources polled</div>
      </div>
      <div style={{ padding: "34px 36px", minWidth: 0 }}>
        <div style={{ fontSize: 20, fontWeight: 600, marginBottom: 4 }}>Reaching consensus</div>
        <div style={{ color: c.dim2, fontSize: 13.5, marginBottom: 22 }}>Independent sources attest to the on-chain facts. A proof is signed only once a supermajority agrees and the block is finalized.</div>
        <div style={{ display: "flex", justifyContent: "space-between", fontFamily: mono, fontSize: 12, color: c.mute, marginBottom: 8 }}>
          <span>ATTESTATIONS</span><span><b style={{ color: c.accent }}>{agreed}</b> / {total} · quorum {quorum} · min {health?.minResponders ?? 7}</span>
        </div>
        <div style={{ height: 10, background: c.panel, border: `1px solid ${c.border2}`, borderRadius: 6, overflow: "hidden", marginBottom: 22 }}>
          <div style={{ height: "100%", background: `linear-gradient(90deg,${c.accentDim},${c.accent})`, width: `${pct}%`, transition: "width .3s" }} />
        </div>
        <div style={{ fontFamily: mono, fontSize: 12.5, lineHeight: 1.95, color: c.dim2, minHeight: 150, maxHeight: 240, overflowY: "auto" }}>
          {sources.map((name: string) => {
            const r: SourceReport | undefined = reports[name];
            if (!r) return <div key={name} style={{ color: c.mute, opacity: 0.6 }}>· {name} <span style={{ color: c.warn }}>polling…</span></div>;
            const good = r.ok && r.found;
            return (
              <div key={name}>
                <span style={{ color: c.mute }}>{String(r.ms).padStart(4)}ms</span> {name}{" "}
                {good ? <span style={{ color: c.accent }}>attested ✓</span> : r.ok ? <span style={{ color: c.mute }}>not found ·</span> : <span style={{ color: c.danger }}>timeout ✕</span>}
              </div>
            );
          })}
          {streamErr && <div style={{ color: c.danger }}>error: {streamErr}</div>}
          {summary && !summary.reached && <div style={{ color: c.warn, marginTop: 8 }}>{summary.reason}</div>}
        </div>
        <div style={{ display: "flex", gap: 12, marginTop: 22, fontFamily: mono, fontSize: 12.5 }}>
          <div onClick={goCompose} style={{ cursor: "pointer", color: c.mute, border: `1px solid ${c.border2}`, borderRadius: 7, padding: "11px 16px" }}>← compose</div>
          <div onClick={proofReady ? goProof : undefined} style={{ cursor: proofReady ? "pointer" : "default", borderRadius: 7, padding: "11px 20px", fontWeight: 700, ...(proofReady ? { color: c.bg, background: c.accent } : { color: c.faint, background: c.panel, border: `1px solid ${c.border2}` }) }}>view proof →</div>
        </div>
      </div>
    </div>
  );
}

/* ---------- PROOF ---------- */
function ProofView({ proof, sigCheck, claim, goConsensus, goHandoff }: { proof: Proof; sigCheck: any; claim: string; goConsensus: () => void; goHandoff: () => void }) {
  const f = proof.finality;
  const copy = () => navigator.clipboard?.writeText(JSON.stringify(proof, null, 2));
  const download = () => {
    const blob = new Blob([JSON.stringify(proof, null, 2)], { type: "application/json" });
    const a = document.createElement("a"); a.href = URL.createObjectURL(blob); a.download = `${shortId(proof.bond?.proofId)}.json`; a.click();
  };
  return (
    <div style={{ maxWidth: 900, margin: "0 auto", padding: "40px 30px", animation: "tsUp .3s ease" }}>
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 20 }}>
        <div><div style={{ fontSize: 20, fontWeight: 600 }}>Resolved proof</div><div style={{ fontFamily: mono, fontSize: 12, color: c.mute, marginTop: 2 }}>AgenticVerificationProof · {proof.schemaVersion}</div></div>
        <Badge ok={proof.verified} text={proof.verified ? "● VERIFIED" : "● " + (proof.reason ? "UNVERIFIED" : "FALSE")} />
      </div>
      {!proof.verified && proof.reason && <div style={{ background: "rgba(224,192,74,.08)", border: `1px solid ${c.warn}`, color: c.warn, borderRadius: 8, padding: "10px 14px", fontFamily: mono, fontSize: 12.5, marginBottom: 18 }}>reason: {proof.reason}</div>}
      <div style={{ display: "grid", gridTemplateColumns: "1fr 250px", gap: 24, alignItems: "start" }}>
        <div style={{ background: c.panel, border: `1px solid ${c.border2}`, borderRadius: 10, padding: "22px 24px", fontFamily: mono, fontSize: 12.5, lineHeight: 1.9 }}>
          <KV k="issuer" v={proof.attestation.signer} />
          <KV k="claim" v={claim || "—"} dim />
          <KV k="txHash" v={proof.txHash} />
          <KV k="block" v={proof.blockNumber ? proof.blockNumber.toLocaleString() : "—"} />
          <KV k="consensus" v={`${proof.consensus.agreed} / ${proof.consensus.sources} · quorum ${proof.consensus.quorum} · ${proof.consensus.responders} live`} accent />
          <KV k="finality" v={f ? (f.finalized ? `finalized · ${f.confirmations} conf` : "NOT finalized") : "—"} accent={f?.finalized} warn={f ? !f.finalized : false} />
          <KV k="issued" v={new Date(proof.issuedAt * 1000).toISOString()} />
          <div style={{ height: 1, background: c.border, margin: "16px 0 12px" }} />
          <div style={{ color: c.mute, fontSize: 11, letterSpacing: ".1em", marginBottom: 6, display: "flex", justifyContent: "space-between" }}>
            <span>ATTESTATION · {proof.attestation.scheme}</span>
            {sigCheck && <span style={{ color: sigCheck.ok ? c.accent : c.danger }}>{sigCheck.ok ? "✓ signature verified in-browser" : "✕ signature invalid"}</span>}
          </div>
          <div style={{ color: c.accent, wordBreak: "break-all", fontSize: 12, lineHeight: 1.6 }}>{proof.attestation.signature}</div>
          {sigCheck?.recovered && <div style={{ color: c.mute, fontSize: 11, marginTop: 6 }}>recovered signer: {sigCheck.recovered}</div>}
        </div>
        <div style={{ display: "flex", flexDirection: "column", gap: 14 }}>
          <ConsensusGrid consensus={proof.consensus} />
          {proof.bond && (
            <div style={{ background: c.panel2, border: `1px solid ${c.border3}`, borderRadius: 10, padding: 16, fontFamily: mono, fontSize: 11.5, color: c.dim2, lineHeight: 1.8 }}>
              <div style={{ color: c.mute, letterSpacing: ".12em", marginBottom: 8 }}>BOND</div>
              <div>stake {proof.bond.stakedUSDC} USDC</div>
              <div>window {proof.bond.challengeWindowSec}s</div>
              <div>anchored {proof.bond.anchored ? "✓" : "—"}</div>
            </div>
          )}
        </div>
      </div>
      <div style={{ display: "flex", gap: 12, marginTop: 18, fontFamily: mono, fontSize: 12.5, flexWrap: "wrap" }}>
        <Btn onClick={copy}>⧉ copy JSON</Btn>
        <Btn onClick={download}>↓ download</Btn>
        <a href={`https://basescan.org/tx/${proof.txHash}`} target="_blank" rel="noreferrer" style={{ background: c.panel, border: `1px solid ${c.border2}`, borderRadius: 7, padding: "10px 15px", color: c.text }}>↗ view tx on-chain</a>
        <div style={{ flex: 1 }} />
        <div onClick={goConsensus} style={{ cursor: "pointer", color: c.mute, border: `1px solid ${c.border2}`, borderRadius: 7, padding: "10px 16px" }}>← consensus</div>
        <div onClick={goHandoff} style={{ cursor: "pointer", color: c.bg, background: c.accent, borderRadius: 7, padding: "10px 20px", fontWeight: 700 }}>hand off →</div>
      </div>
    </div>
  );
}

/* ---------- HANDOFF ---------- */
function Handoff({ proof, counterparty, setCounterparty, anchorState, anchorTx, anchorErr, health, confirmHandoff, goProof, newVerification, openMap }: any) {
  if (anchorState === "done") {
    return (
      <div style={{ maxWidth: 640, margin: "0 auto", padding: "44px 30px", textAlign: "center", animation: "tsUp .35s ease" }}>
        <div style={{ width: 70, height: 70, borderRadius: "50%", border: `2px solid ${c.accent}`, display: "flex", alignItems: "center", justifyContent: "center", margin: "0 auto 20px", boxShadow: "0 0 26px rgba(182,255,61,.3)" }}>
          <div style={{ width: 20, height: 11, border: `3px solid ${c.accent}`, borderTop: 0, borderRight: 0, transform: "rotate(-45deg)", marginTop: -6 }} />
        </div>
        <div style={{ fontSize: 22, fontWeight: 600, marginBottom: 8 }}>{anchorTx ? "Proof anchored & handed off" : "Proof handed off"}</div>
        <div style={{ fontFamily: mono, fontSize: 13, color: c.dim2, marginBottom: 6 }}><span style={{ color: c.accent }}>{shortId(proof.bond?.proofId)}</span> → {counterparty}</div>
        {anchorTx ? (
          <div style={{ fontFamily: mono, fontSize: 12, marginBottom: 26 }}>anchored on Base · <a href={`https://basescan.org/tx/${anchorTx}`} target="_blank" rel="noreferrer">{anchorTx.slice(0, 18)}… ↗</a></div>
        ) : (
          <div style={{ fontFamily: mono, fontSize: 12, color: c.mute, marginBottom: 26 }}>exported signed proof (bonding off — set BOND_ENABLED to anchor on-chain)</div>
        )}
        <div style={{ display: "flex", gap: 12, justifyContent: "center", fontFamily: mono, fontSize: 12.5 }}>
          <div onClick={newVerification} style={{ cursor: "pointer", color: c.text, border: `1px solid ${c.border2}`, borderRadius: 7, padding: "12px 18px" }}>+ new verification</div>
          <div onClick={openMap} style={{ cursor: "pointer", color: c.bg, background: c.accent, borderRadius: 7, padding: "12px 20px", fontWeight: 700 }}>view on proof map →</div>
        </div>
      </div>
    );
  }
  const canAnchor = health?.bondAnchorReady && !!proof.bond?.proofId;
  return (
    <div style={{ maxWidth: 640, margin: "0 auto", padding: "44px 30px", animation: "tsUp .3s ease" }}>
      <div style={{ fontSize: 20, fontWeight: 600, marginBottom: 4 }}>Hand off the signed proof</div>
      <div style={{ color: c.dim2, fontSize: 13.5, marginBottom: 24 }}>Deliver the proof to your counterparty — they validate it against the signer's public key, no trust in you required. {canAnchor ? "Confirming also anchors it on-chain so anyone can slash it if it's fraudulent." : ""}</div>
      <div style={{ background: c.panel2, border: `1px solid ${canAnchor ? c.accentBorder : c.border3}`, borderRadius: 10, padding: 18, marginBottom: 22, fontFamily: mono, fontSize: 12.5, color: c.dim2, lineHeight: 1.9 }}>
        <div style={{ color: c.mute, letterSpacing: ".1em", marginBottom: 8 }}>ON-CHAIN BOND</div>
        {canAnchor ? <>
          <div>contract <span style={{ color: c.text }}>{health.bondContract.slice(0, 16)}…</span></div>
          <div>this proof anchors <span style={{ color: c.accent }}>a slashable stake</span> for {proof.bond.challengeWindowSec}s</div>
        </> : <div style={{ color: c.mute }}>bonding not configured — handoff will export the signed proof only.</div>}
      </div>
      <Field label="COUNTERPARTY DID">
        <input value={counterparty} onChange={(e) => setCounterparty(e.target.value)} style={{ width: "100%", background: c.panel, border: `1px solid ${c.border2}`, borderRadius: 8, padding: "12px 14px", fontSize: 13, color: c.text }} />
      </Field>
      {anchorErr && <div style={{ color: c.danger, fontFamily: mono, fontSize: 12, marginTop: 12 }}>{anchorErr}</div>}
      <div style={{ display: "flex", gap: 12, marginTop: 24, fontFamily: mono, fontSize: 12.5 }}>
        <div onClick={goProof} style={{ cursor: "pointer", color: c.mute, border: `1px solid ${c.border2}`, borderRadius: 7, padding: "12px 16px" }}>← proof</div>
        <div onClick={anchorState === "anchoring" ? undefined : confirmHandoff} style={{ cursor: anchorState === "anchoring" ? "default" : "pointer", flex: 1, textAlign: "center", color: c.bg, background: c.accent, borderRadius: 7, padding: 12, fontWeight: 700, opacity: anchorState === "anchoring" ? 0.7 : 1 }}>
          {anchorState === "anchoring" ? "anchoring on Base…" : canAnchor ? "→ anchor + confirm handoff" : "→ confirm handoff"}
        </div>
      </div>
    </div>
  );
}

/* ---------- MAP ---------- */
function MapView({ proofs, selectedId, setSelectedId, newVerification }: any) {
  const sel: StoredProof | undefined = proofs.find((p: StoredProof) => p.proofId === selectedId) || proofs[0];
  return (
    <div style={{ flex: 1, position: "relative", overflow: "hidden", background: "radial-gradient(circle at 42% 52%,rgba(182,255,61,.05),transparent 55%)" }}>
      <ProofGraph proofs={proofs} selectedId={selectedId} onSelect={setSelectedId} />
      <div style={{ position: "absolute", left: 24, top: 24, fontFamily: mono, fontSize: 11, color: c.mute, lineHeight: 1.7 }}>
        <div style={{ color: c.text, fontSize: 12.5, fontWeight: 600, letterSpacing: ".1em" }}>PROOF MAP</div>
        {proofs.length} proofs · {proofs.filter((p: StoredProof) => !p.verified).length} unresolved<br />click a node to inspect
      </div>
      {sel && (
        <div style={{ position: "absolute", right: 26, top: 26, width: 320, background: "rgba(18,22,26,.94)", backdropFilter: "blur(8px)", border: `1px solid ${c.accentBorder}`, borderRadius: 12, padding: "20px 22px", boxShadow: "0 16px 50px rgba(0,0,0,.6)" }}>
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 14 }}>
            <span style={{ fontFamily: mono, fontSize: 13.5, color: c.accent }}>{shortId(sel.proofId)}</span>
            <Badge ok={sel.verified} text={sel.verified ? "VERIFIED" : "UNVERIFIED"} small />
          </div>
          <div style={{ fontFamily: mono, fontSize: 11.5, lineHeight: 2, color: c.dim2 }}>
            <div><span style={{ color: c.mute }}>claim </span>{sel.claim || "—"}</div>
            <div><span style={{ color: c.mute }}>tx </span>{sel.proof.txHash.slice(0, 20)}…</div>
            <div><span style={{ color: c.mute }}>cons. </span>{sel.proof.consensus.agreed}/{sel.proof.consensus.sources} · quorum {sel.proof.consensus.quorum}</div>
            <div><span style={{ color: c.mute }}>final </span>{sel.proof.finality?.finalized ? `yes · ${sel.proof.finality.confirmations} conf` : "no"}</div>
            <div><span style={{ color: c.mute }}>bond </span>{sel.proof.bond?.anchored ? "anchored ✓" : sel.proof.bond ? "advertised" : "—"}</div>
          </div>
          <div style={{ height: 1, background: c.border2, margin: "15px 0 13px" }} />
          <a href={`https://basescan.org/tx/${sel.proof.txHash}`} target="_blank" rel="noreferrer" style={{ display: "block", textAlign: "center", color: c.bg, background: c.accent, borderRadius: 6, padding: 9, fontFamily: mono, fontSize: 11.5, fontWeight: 600 }}>view tx on-chain ↗</a>
        </div>
      )}
      <div style={{ position: "absolute", left: "50%", bottom: 26, transform: "translateX(-50%)", display: "flex", alignItems: "center", gap: 6, background: "rgba(18,22,26,.94)", backdropFilter: "blur(8px)", border: `1px solid ${c.border2}`, borderRadius: 13, padding: "8px 10px", boxShadow: "0 12px 40px rgba(0,0,0,.55)", fontFamily: mono, fontSize: 12 }}>
        <span onClick={newVerification} style={{ cursor: "pointer", background: c.accent, color: c.bg, borderRadius: 7, padding: "9px 14px", fontWeight: 600 }}>+ verify</span>
      </div>
    </div>
  );
}

/* ---------- shared bits ---------- */
function Logo() {
  return <div style={{ display: "grid", gridTemplateColumns: "9px 9px", gridTemplateRows: "9px 9px", gap: 2 }}><div style={{ background: c.accent }} /><div style={{ border: `1px solid ${c.border2}` }} /><div style={{ border: `1px solid ${c.border2}` }} /><div style={{ background: c.text }} /></div>;
}
function RailStep({ label, idx, cur, last, onClick }: { label: string; idx: number; cur: number; last: boolean; onClick: () => void }) {
  const st = idx < cur ? "done" : idx === cur ? "active" : "todo";
  const circle: React.CSSProperties = { width: 23, height: 23, borderRadius: "50%", display: "inline-flex", alignItems: "center", justifyContent: "center", fontSize: 11, flex: "none",
    ...(st === "done" ? { background: c.panel, border: `1px solid ${c.accentBorder}`, color: c.accent } : st === "active" ? { background: c.accent, color: c.bg, boxShadow: "0 0 12px rgba(182,255,61,.5)" } : { border: `1px solid ${c.border2}`, color: c.faint }) };
  return (
    <>
      <div onClick={onClick} style={{ display: "flex", alignItems: "center", gap: 9, cursor: "pointer", color: st === "active" ? c.text : st === "done" ? c.mute : c.faint, fontWeight: st === "active" ? 600 : 400 }}>
        <span style={circle}>{st === "done" ? "✓" : st === "active" ? "◉" : "○"}</span>{label}
      </div>
      {!last && <div style={{ flex: 1, height: 2, margin: "0 12px", background: idx < cur ? `linear-gradient(90deg,${c.accent},${c.accentBorder})` : `repeating-linear-gradient(90deg,${c.border2} 0 6px,transparent 6px 12px)` }} />}
    </>
  );
}
function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return <div><label style={{ fontFamily: mono, fontSize: 11, color: c.mute, letterSpacing: ".08em" }}>{label}</label><div style={{ marginTop: 8 }}>{children}</div></div>;
}
function Chip({ label, value, accent }: { label: string; value: string; accent?: boolean }) {
  return <div><label style={{ fontFamily: mono, fontSize: 11, color: c.mute, letterSpacing: ".08em" }}>{label}</label>
    <div style={{ marginTop: 8, background: c.panel, border: `1px solid ${c.border2}`, borderRadius: 8, padding: "12px 14px", fontFamily: mono, fontSize: 12.5, color: accent ? c.accent : c.dim, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{value}</div></div>;
}
function KV({ k, v, dim, accent, warn }: { k: string; v: string; dim?: boolean; accent?: boolean; warn?: boolean }) {
  return <div style={{ display: "grid", gridTemplateColumns: "92px 1fr", gap: "6px 14px", marginBottom: 6 }}>
    <span style={{ color: c.mute }}>{k}</span><span style={{ color: warn ? c.warn : accent ? c.accent : dim ? c.dim : c.text, wordBreak: "break-all" }}>{v}</span></div>;
}
function ConsensusGrid({ consensus }: { consensus: Proof["consensus"] }) {
  const total = consensus.sources || 11;
  const agreed = consensus.agreed;
  return (
    <div style={{ background: c.panel2, border: `1px solid ${c.border3}`, borderRadius: 10, padding: 16 }}>
      <div style={{ fontFamily: mono, fontSize: 10, color: c.mute, letterSpacing: ".14em", marginBottom: 12 }}>CONSENSUS {agreed}/{total}</div>
      <div style={{ display: "grid", gridTemplateColumns: "repeat(4,1fr)", gap: 5 }}>
        {Array.from({ length: total }, (_, i) => <div key={i} style={{ aspectRatio: "1", background: i < agreed ? c.accent : c.border3, borderRadius: 3 }} />)}
      </div>
    </div>
  );
}
function Badge({ ok, text, small }: { ok: boolean; text: string; small?: boolean }) {
  const col = ok ? c.accent : c.warn;
  return <span style={{ fontFamily: mono, fontSize: small ? 10 : 11, color: col, border: `1px solid ${col}`, borderRadius: small ? 5 : 6, padding: small ? "2px 7px" : "5px 11px", letterSpacing: ".1em" }}>{text}</span>;
}
function Btn({ onClick, children }: { onClick: () => void; children: React.ReactNode }) {
  return <div onClick={onClick} style={{ cursor: "pointer", background: c.panel, border: `1px solid ${c.border2}`, borderRadius: 7, padding: "10px 15px", color: c.text }}>{children}</div>;
}
function shortId(id?: string) { return id ? "avp_" + id.replace(/^0x/, "").slice(0, 6) : "avp"; }
