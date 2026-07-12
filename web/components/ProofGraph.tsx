"use client";
import { useEffect, useRef } from "react";
import type { StoredProof } from "@/lib/types";

// Orbital map of session proofs around the TESSERA core; click a node to inspect.
export default function ProofGraph({ proofs, selectedId, onSelect }: {
  proofs: StoredProof[];
  selectedId: string;
  onSelect: (id: string) => void;
}) {
  const ref = useRef<HTMLCanvasElement>(null);
  const state = useRef({ proofs, selectedId });
  state.current = { proofs, selectedId };
  const orbit = useRef<Record<string, { ang: number; rad: number; spd: number; dir: number }>>({});

  useEffect(() => {
    const canvas = ref.current;
    if (!canvas) return;
    const ctx = canvas.getContext("2d")!;
    let alive = true;
    const fit = () => {
      const dpr = window.devicePixelRatio || 1;
      const r = canvas.getBoundingClientRect();
      const W = r.width || 800, H = r.height || 500;
      canvas.width = Math.round(W * dpr); canvas.height = Math.round(H * dpr);
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
      return { W, H };
    };
    let dim = fit();
    const onResize = () => (dim = fit());
    window.addEventListener("resize", onResize);
    let hit: { id: string; x: number; y: number }[] = [];
    const onClick = (e: MouseEvent) => {
      const r = canvas.getBoundingClientRect();
      const mx = e.clientX - r.left, my = e.clientY - r.top;
      let best: string | null = null, bd = 1e9;
      hit.forEach((h) => { const d = Math.hypot(h.x - mx, h.y - my); if (d < bd) { bd = d; best = h.id; } });
      if (best && bd < 26) onSelect(best);
    };
    canvas.addEventListener("click", onClick);
    const orbitFor = (id: string, i: number) => {
      if (!orbit.current[id]) {
        orbit.current[id] = { ang: (i * 2.399963) % (Math.PI * 2), rad: 0.17 + ((i * 37) % 60) / 100 * 0.42, spd: 0.03 + ((i * 13) % 7) / 7 * 0.06, dir: i % 2 === 0 ? 1 : -1 };
      }
      return orbit.current[id];
    };
    const col: Record<string, string> = { v: "#b6ff3d", f: "#e0c04a" };
    let t = 0;
    const frame = () => {
      if (!alive) return;
      t += 0.0045;
      const { W, H } = dim;
      const cx = W * 0.44, cy = H * 0.52, base = Math.min(W, H);
      ctx.clearRect(0, 0, W, H);
      const { proofs: ps, selectedId: sel } = state.current;
      hit = [];
      const P = ps.map((p, i) => {
        const o = orbitFor(p.proofId || String(i), i);
        const a = o.ang + t * o.spd * o.dir;
        return { id: p.proofId, st: p.verified ? "v" : "f", x: cx + Math.cos(a) * o.rad * base * 1.15, y: cy + Math.sin(a) * o.rad * base * 0.9 };
      });
      P.forEach((p) => {
        ctx.strokeStyle = p.st === "f" ? "rgba(224,192,74,.24)" : "rgba(182,255,61,.18)";
        ctx.lineWidth = 1; ctx.beginPath(); ctx.moveTo(cx, cy); ctx.lineTo(p.x, p.y); ctx.stroke();
      });
      ctx.font = "10.5px 'JetBrains Mono', monospace";
      P.forEach((p) => {
        const isSel = p.id === sel;
        const cc = col[p.st] || "#b6ff3d";
        if (isSel) { ctx.strokeStyle = "rgba(182,255,61,.55)"; ctx.lineWidth = 1.5; ctx.beginPath(); ctx.arc(p.x, p.y, 15 + Math.sin(t * 3) * 1.5, 0, 7); ctx.stroke(); }
        ctx.beginPath(); ctx.arc(p.x, p.y, isSel ? 7 : 5.5, 0, 7);
        ctx.fillStyle = cc; ctx.shadowColor = cc; ctx.shadowBlur = isSel ? 16 : 8; ctx.fill(); ctx.shadowBlur = 0;
        ctx.fillStyle = isSel ? "rgba(232,237,233,.9)" : "rgba(232,237,233,.5)";
        ctx.fillText(shortId(p.id), p.x + 11, p.y + 3.5);
        hit.push({ id: p.id, x: p.x, y: p.y });
      });
      ctx.beginPath(); ctx.arc(cx, cy, 13, 0, 7); ctx.strokeStyle = "rgba(182,255,61,.5)"; ctx.lineWidth = 1.5; ctx.stroke();
      ctx.beginPath(); ctx.arc(cx, cy, 8, 0, 7); ctx.fillStyle = "#eef0f4"; ctx.shadowColor = "#b6ff3d"; ctx.shadowBlur = 22; ctx.fill(); ctx.shadowBlur = 0;
      ctx.fillStyle = "rgba(232,237,233,.85)"; ctx.font = "600 10.5px 'JetBrains Mono', monospace";
      ctx.fillText("TESSERA core", cx + 17, cy + 3.5);
      requestAnimationFrame(frame);
    };
    frame();
    return () => { alive = false; window.removeEventListener("resize", onResize); canvas.removeEventListener("click", onClick); };
  }, [onSelect]);
  return <canvas ref={ref} style={{ position: "absolute", inset: 0, width: "100%", height: "100%" }} />;
}

function shortId(id: string) {
  if (!id) return "avp";
  return "avp_" + id.replace(/^0x/, "").slice(0, 6);
}
