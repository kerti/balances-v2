import { lazy, type ComponentType } from 'react'

// Key under which we record that we've already force-reloaded once this tab
// session to recover a failed chunk import. sessionStorage (not localStorage)
// so the guard resets when the tab closes.
export const RELOAD_GUARD_KEY = 'balances.chunkReloaded'

// A stale, content-hashed chunk that no longer exists after a deploy rejects
// the dynamic import() with one of these engine-specific messages. (#190 makes
// the missing chunk answer a clean 404 instead of the SPA shell; this matches
// the resulting rejection.)
export function isChunkLoadError(err: unknown): boolean {
  const message = err instanceof Error ? err.message : String(err)
  return /failed to fetch dynamically imported module|error loading dynamically imported module|importing a module script failed/i.test(
    message,
  )
}

// The side-effecting bits the reload guard needs, injected so the decision
// logic is unit-testable without a DOM (the suite runs in the `node`
// environment — see vitest.config).
export type ReloadEnv = {
  guardSet: () => boolean
  setGuard: () => void
  clearGuard: () => void
  reload: () => void
}

// Real browser env. sessionStorage access is wrapped because it throws when
// storage is blocked (private mode, embedded contexts); the guard is
// best-effort, so an unreadable store reads as "not set" and worst case costs
// one extra reload, never a tighter loop.
const browserEnv: ReloadEnv = {
  guardSet() {
    try {
      return sessionStorage.getItem(RELOAD_GUARD_KEY) === '1'
    } catch {
      return false
    }
  },
  setGuard() {
    try {
      sessionStorage.setItem(RELOAD_GUARD_KEY, '1')
    } catch {
      /* ignore — see guardSet */
    }
  },
  clearGuard() {
    try {
      sessionStorage.removeItem(RELOAD_GUARD_KEY)
    } catch {
      /* ignore — see guardSet */
    }
  },
  reload() {
    window.location.reload()
  },
}

// Clears the one-shot reload guard. Called from the route error boundary's
// manual Reload so an explicit retry gets a fresh auto-reload attempt.
export function clearReloadGuard(): void {
  browserEnv.clearGuard()
}

// Wraps a dynamic-import factory so a post-deploy chunk-load failure recovers
// silently: on the first such failure this session we force a single reload
// onto the new bundle and hold the Suspense fallback through the navigation. A
// chunk that is genuinely gone for good reloads at most once, then rethrows to
// the route error boundary instead of looping. A clean load clears the guard so
// the next deploy gets its own one-shot. `env` is injected for tests; callers
// use the default browser env.
export async function importWithReloadGuard<T>(
  factory: () => Promise<T>,
  env: ReloadEnv = browserEnv,
): Promise<T> {
  try {
    const mod = await factory()
    env.clearGuard()
    return mod
  } catch (err) {
    if (isChunkLoadError(err) && !env.guardSet()) {
      env.setGuard()
      env.reload()
      // Never settles before the navigation — keeps the Suspense fallback up
      // rather than flashing the error boundary for the split second before
      // the reload takes effect.
      return new Promise<T>(() => {})
    }
    throw err
  }
}

// lazy() drop-in that adds the post-deploy chunk-reload recovery above.
export function lazyWithReload<P extends object>(
  factory: () => Promise<{ default: ComponentType<P> }>,
) {
  return lazy(() => importWithReloadGuard(factory))
}
