"use client";
import { useEffect, useRef } from "react";

// Rotating sphere of N verifier nodes; `lit[i]` true = that source has agreed.
// Ported from the console comp's ts-net-live, driven by real source events.
export default function VerifierSphere({ lit, style }: { lit: boolean[]; style?: React.CSSProperties }) {
  const ref = useRef<HTMLCanvasElement>(null);
  const litRef = useRef(lit);
  litRef.current = lit;
  useEffect(() => {
    const canvas = ref.current;
    if (!canvas) return;
    const ctx = canvas.getContext("2d")!;
    let alive = true;
    const fit = () => {
      const dpr = window.devicePixelRatio || 1;
      const r = canvas.getBoundingClientRect();
      const W = r.width || 360, H = r.height || 320;
      canvas.width = Math.round(W * dpr); canvas.height = Math.round(H * dpr);
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
      return { W, H };
    };
    let dim = fit();
    const onResize = () => (dim = fit());
    window.addEventListener("resize", onResize);
    const N = Math.max(1, litRef.current.length);
    const nodes = Array.from({ length: N }, (_, i) => {
      const y = 1 - (i / (N - 1)) * 2;
      const rad = Math.sqrt(Math.max(0, 1 - y * y));
      const theta = i * 2.399963;
      return { x: Math.cos(theta) * rad, y, z: Math.sin(theta) * rad };
    });
    let t = 0;
    const frame = () => {
      if (!alive) return;
      t += 0.006;
      const litArr = litRef.current;
      const { W, H } = dim;
      const cx = W / 2, cy = H / 2, R = Math.min(W, H) * 0.36, cam = 2.6, tilt = 0.4;
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
        const on = litArr[i];
        ctx.strokeStyle = on ? `rgba(182,255,61,${0.1 + 0.22 * p.s})` : `rgba(125,138,130,${0.05 + 0.09 * p.s})`;
        ctx.lineWidth = on ? 1.1 : 0.7;
        ctx.beginPath(); ctx.moveTo(cx, cy); ctx.lineTo(p.sx, p.sy); ctx.stroke();
        if (on) { const ph = (t * 0.6 + i * 0.13) % 1; ctx.fillStyle = `rgba(182,255,61,${0.75 * ph})`; ctx.beginPath(); ctx.arc(cx + (p.sx - cx) * (1 - ph), cy + (p.sy - cy) * (1 - ph), 1.7, 0, 7); ctx.fill(); }
      });
      pts.map((p, i) => ({ p, i })).sort((a, b) => a.p.z - b.p.z).forEach(({ p, i }) => {
        const on = litArr[i];
        ctx.beginPath(); ctx.arc(p.sx, p.sy, Math.max(0.5, (on ? 3.4 : 2.6) * p.s), 0, 7);
        if (on) { ctx.fillStyle = "#b6ff3d"; ctx.shadowColor = "#b6ff3d"; ctx.shadowBlur = 8 * p.s; } else { ctx.fillStyle = "#7d8a82"; ctx.shadowBlur = 0; }
        ctx.fill(); ctx.shadowBlur = 0;
      });
      ctx.beginPath(); ctx.arc(cx, cy, 6, 0, 7); ctx.fillStyle = "#eef0f4"; ctx.shadowColor = "#b6ff3d"; ctx.shadowBlur = 18; ctx.fill(); ctx.shadowBlur = 0;
      ctx.strokeStyle = "rgba(182,255,61,.45)"; ctx.lineWidth = 1; ctx.beginPath(); ctx.arc(cx, cy, 10, 0, 7); ctx.stroke();
      requestAnimationFrame(frame);
    };
    frame();
    return () => { alive = false; window.removeEventListener("resize", onResize); };
  }, []);
  return <canvas ref={ref} style={style} />;
}
