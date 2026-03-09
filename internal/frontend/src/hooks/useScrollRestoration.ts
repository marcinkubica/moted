import { useCallback, useEffect, useRef } from "react";

const SESSION_KEY = "mo-scroll-context";

interface ScrollContext {
  headingId: string | null;
  relativeOffset: number;
  rawScrollTop: number;
  fileId: string;
  url: string;
}

export function useScrollRestoration(
  scrollContainer: HTMLElement | null,
  activeHeadingId: string | null,
  activeFileId: string | null,
) {
  const savedContextRef = useRef<ScrollContext | null>(null);
  const pendingRestoreRef = useRef(false);

  // Keep latest values in refs for beforeunload (which can't use stale closures)
  const scrollContainerRef = useRef(scrollContainer);
  scrollContainerRef.current = scrollContainer;
  const activeHeadingIdRef = useRef(activeHeadingId);
  activeHeadingIdRef.current = activeHeadingId;
  const activeFileIdRef = useRef(activeFileId);
  activeFileIdRef.current = activeFileId;

  const captureScrollPosition = useCallback(() => {
    const sc = scrollContainerRef.current;
    const fileId = activeFileIdRef.current;
    if (!sc || !fileId) return;

    const headingId = activeHeadingIdRef.current;
    const rawScrollTop = sc.scrollTop;
    let relativeOffset = 0;

    if (headingId) {
      const headingEl = document.getElementById(headingId);
      if (headingEl) {
        relativeOffset =
          headingEl.getBoundingClientRect().top -
          sc.getBoundingClientRect().top;
      }
    }

    const ctx: ScrollContext = {
      headingId,
      relativeOffset,
      rawScrollTop,
      fileId,
      url: window.location.pathname,
    };

    savedContextRef.current = ctx;
    pendingRestoreRef.current = true;

    try {
      sessionStorage.setItem(SESSION_KEY, JSON.stringify(ctx));
    } catch {
      // sessionStorage may be unavailable
    }
  }, []);

  const restoreFromContext = useCallback(
    (ctx: ScrollContext) => {
      if (!scrollContainer) return;

      if (ctx.headingId) {
        const headingEl = document.getElementById(ctx.headingId);
        if (headingEl) {
          const currentOffset =
            headingEl.getBoundingClientRect().top -
            scrollContainer.getBoundingClientRect().top;
          scrollContainer.scrollTop += currentOffset - ctx.relativeOffset;
          return;
        }
      }

      scrollContainer.scrollTop = ctx.rawScrollTop;
    },
    [scrollContainer],
  );

  const onContentRendered = useCallback(() => {
    // Path A: React re-render (ref-based)
    if (pendingRestoreRef.current && savedContextRef.current) {
      const ctx = savedContextRef.current;
      if (ctx.fileId === activeFileId) {
        restoreFromContext(ctx);
      }
      savedContextRef.current = null;
      pendingRestoreRef.current = false;
      try {
        sessionStorage.removeItem(SESSION_KEY);
      } catch {
        // ignore
      }
      return;
    }

    // Path B: Full page reload (sessionStorage-based)
    try {
      const stored = sessionStorage.getItem(SESSION_KEY);
      if (stored) {
        const ctx: ScrollContext = JSON.parse(stored);
        sessionStorage.removeItem(SESSION_KEY);
        if (
          ctx.fileId === activeFileId &&
          ctx.url === window.location.pathname
        ) {
          restoreFromContext(ctx);
        }
      }
    } catch {
      // ignore
    }
  }, [activeFileId, restoreFromContext]);

  // Capture scroll position before any page unload
  useEffect(() => {
    const handler = () => captureScrollPosition();
    window.addEventListener("beforeunload", handler);
    return () => window.removeEventListener("beforeunload", handler);
  }, [captureScrollPosition]);

  return { captureScrollPosition, onContentRendered };
}
