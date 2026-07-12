"use client";
import { useEffect, useRef } from "react";

// A slowly-rotating node constellation with a bright core — the "verifier
// network" motif behind the hero. Pure canvas, no deps.
export default function HeroNet({ style }: { style?: React.CSSProperties }) {
  const ref = useRef<HTMLCanvasElement>(null);
  useEffect(() => {
    const canvas = ref.current;
    if (!canvas) return;
    const ctx = canvas.getContext("2d")!;
    let alive = true;
    const fit = () => {
      const dpr = window.devicePixelRatio || 1;
      const r = canvas.getBoundingClientRect();
      const W = r.width || 760, H = r.height || 700;
      canvas.width = Math.round(W * dpr);
      canvas.height = Math.round(H * dpr);
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
      return { W, H };
    };
    let dim = fit();
    const onResize = () => (dim = fit());
    window.addEventListener("resize", onResize);
    const N = 13, nodes: { x: number; y: number; z: number }[] = [];
    for (let i = 0; i < N; i++) {
      const y = 1 - (i / (N - 1)) * 2;
      const rad = Math.sqrt(Math.max(0, 1 - y * y));
      const theta = i * 2.399963;
      nodes.push({ x: Math.cos(theta) * rad, y, z: Math.sin(theta) * rad });
    }
    let t = 0;
    const frame = () => {
      if (!alive) return;
      t += 0.004;
      const { W, H } = dim;
      const cx = W * 0.55, cy = H * 0.46, R = Math.min(W, H) * 0.32, cam = 2.6, tilt = 0.42;
      ctx.clearRect(0, 0, W, H);
      const cosA = Math.cos(t), sinA = Math.sin(t);
      const pts = nodes.map((n) => {
        let x = n.x * cosA - n.z * sinA;
        let z = n.x * sinA + n.z * cosA;
        const y = n.y * Math.cos(tilt) - z * Math.sin(tilt);
        z = n.y * Math.sin(tilt) + z * Math.cos(tilt);
        const s = cam / (cam + z);
        return { sx: cx + x * R * s, sy: cy + y * R * s, s, z };
      });
      pts.forEach((p, i) => {
        ctx.strokeStyle = `rgba(182,255,61,${0.05 + 0.16 * p.s})`;
        ctx.lineWidth = 0.8;
        ctx.beginPath(); ctx.moveTo(cx, cy); ctx.lineTo(p.sx, p.sy); ctx.stroke();
        const ph = (t * 0.5 + i * 0.11) % 1;
        ctx.fillStyle = `rgba(182,255,61,${0.6 * ph})`;
        ctx.beginPath(); ctx.arc(cx + (p.sx - cx) * (1 - ph), cy + (p.sy - cy) * (1 - ph), 1.6, 0, 7); ctx.fill();
      });
      pts.map((p, i) => ({ p, i })).sort((a, b) => a.p.z - b.p.z).forEach(({ p }) => {
        ctx.beginPath(); ctx.arc(p.sx, p.sy, Math.max(0.6, 3.2 * p.s), 0, 7);
        ctx.fillStyle = "#b6ff3d"; ctx.shadowColor = "#b6ff3d"; ctx.shadowBlur = 8 * p.s;
        ctx.fill(); ctx.shadowBlur = 0;
      });
      ctx.beginPath(); ctx.arc(cx, cy, 6, 0, 7); ctx.fillStyle = "#eef0f4";
      ctx.shadowColor = "#b6ff3d"; ctx.shadowBlur = 18; ctx.fill(); ctx.shadowBlur = 0;
      requestAnimationFrame(frame);
    };
    frame();
    return () => { alive = false; window.removeEventListener("resize", onResize); };
  }, []);
  return <canvas ref={ref} style={style} />;
}
