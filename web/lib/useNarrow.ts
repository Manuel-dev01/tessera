"use client";
import { useEffect, useState } from "react";

// useNarrow reports whether the viewport is at or below `bp` px. It starts false
// (desktop) so SSR and the first client render match (no hydration mismatch),
// then updates after mount and on resize. Used to collapse the app's inline-style
// grids/rows on mobile, where there are no CSS media queries.
export function useNarrow(bp = 760): boolean {
  const [narrow, setNarrow] = useState(false);
  useEffect(() => {
    const on = () => setNarrow(window.innerWidth <= bp);
    on();
    window.addEventListener("resize", on);
    return () => window.removeEventListener("resize", on);
  }, [bp]);
  return narrow;
}
