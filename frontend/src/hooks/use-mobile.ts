import * as React from "react";

const MOBILE_BREAKPOINT = 768;

// useSyncExternalStore instead of the shadcn default (useEffect + setState):
// a media query is exactly an external store, and this keeps the repo's
// react-hooks/set-state-in-effect rule satisfied without an eslint-disable.
//
// We subscribe to BOTH the media-query `change` event and window `resize`.
// Chromium DevTools device-mode toggling sometimes fails to fire the
// matchMedia `change` event, which would leave this value stuck across the
// breakpoint and make shadcn's Sidebar render its (closed) mobile Sheet at
// desktop width — the sidebar "disappears" until a refresh. `resize` always
// fires on a viewport change, so it's the reliable fallback; getSnapshot
// returns a boolean, so re-renders only happen when the breakpoint actually
// flips, not on every resize tick.
export function useIsMobile() {
  const subscribe = React.useCallback((onStoreChange: () => void) => {
    const mql = window.matchMedia(`(max-width: ${MOBILE_BREAKPOINT - 1}px)`);
    mql.addEventListener("change", onStoreChange);
    window.addEventListener("resize", onStoreChange);
    return () => {
      mql.removeEventListener("change", onStoreChange);
      window.removeEventListener("resize", onStoreChange);
    };
  }, []);

  return React.useSyncExternalStore(
    subscribe,
    () => window.innerWidth < MOBILE_BREAKPOINT,
    () => false,
  );
}
