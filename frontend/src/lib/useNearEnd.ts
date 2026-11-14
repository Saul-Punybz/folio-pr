import { useEffect, useRef } from 'react';

/** Llama a onNear cuando el elemento centinela se acerca a la pantalla. */
export function useNearEnd(onNear: () => void, enabled: boolean) {
  const ref = useRef<HTMLDivElement>(null);
  const cb = useRef(onNear);
  cb.current = onNear;
  useEffect(() => {
    const el = ref.current;
    if (!enabled || !el || typeof IntersectionObserver === 'undefined') return;
    const io = new IntersectionObserver(
      (entries) => { if (entries.some((e) => e.isIntersecting)) cb.current(); },
      { rootMargin: '400px' }
    );
    io.observe(el);
    return () => io.disconnect();
  }, [enabled]);
  return ref;
}
