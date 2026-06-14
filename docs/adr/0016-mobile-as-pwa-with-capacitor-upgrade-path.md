# Mobile as a PWA, with Capacitor as the upgrade path

Mobile delivery is a **Progressive Web App** built on the existing React codebase. No separate
mobile codebase is maintained. If app-store presence or richer native API access becomes a real need
later, **Capacitor** wraps the same web app in a native shell with a few days of work — no rewrite.

## What the PWA path delivers

- **Same React codebase** as the web app (ADR-0015). A `manifest.json` describes the app for "Add to
  Home Screen" install; a service worker handles installability and basic asset caching.
- **iOS install** via Safari → Share → Add to Home Screen. Behaves like a native app once installed:
  full-screen, no browser chrome, custom icon.
- **Android install** is even smoother (Chrome offers an install prompt automatically).
- **iOS 16.4+** supports web push notifications, badging, and improved install UX — closes most of
  the historical "PWAs are second-class on iOS" gap.

## Why this is the right starting point

- **Audience is two known users** (the user + spouse). App-store discovery is irrelevant; there's no
  upside to paying $99/yr Apple Developer + $25 Google fees, or sitting through review cycles, for
  an app that no third party will ever see.
- **Workload is forms, tables, charts.** No camera, biometric, file-system, or heavy native API
  needs. Web on mobile handles this entirely.
- **Solo backend developer with near-zero frontend skill** — taking on a second
  mobile codebase or a different rendering model (React Native primitives) would multiply the
  surface area to maintain. PWA reuses everything.
- **iOS-first preference is fine for PWAs now** — modern iOS treats installed PWAs reasonably well.

## When to upgrade to Capacitor

Trigger conditions that would justify the wrapper:
- Wanting App Store presence (polish, brand, parental approval for installs)
- Needing native plugins beyond what PWAs offer (deep system integration, native biometrics that go
  beyond WebAuthn, contacts, calendar, etc.)
- Hitting iOS PWA limitations that affect daily use

Capacitor was chosen as the documented upgrade path (rather than React Native) because it **wraps
the existing web codebase** rather than replacing it. The React + shadcn/ui + TanStack Query app
keeps working inside the WebView; Capacitor adds optional native plugin access on top.

## Considered alternatives

- **Capacitor from day one.** Rejected — pays App Store fees, review delays, and a slightly more
  complex build pipeline for benefits (store presence, plugin access) that are not yet needed. Easy
  to adopt later without rework.
- **React Native (with code-sharing via React Native Web).** Rejected — different UI primitives
  (`<View>` vs `<div>`), different styling model (StyleSheet vs Tailwind), and the cross-platform
  code-sharing story is more aspirational than practical. Maintaining a second UI codebase is a
  large ask for a solo dev with limited frontend experience.
- **Native iOS (Swift) + Android (Kotlin).** Rejected — two additional codebases. Unrealistic for
  solo backend dev.
- **No mobile at all (web only).** Rejected — the original brief requires mobile use; PWA delivers
  it without additional codebases.

## Deferred (additive when needed)

- **Offline support.** Service workers can cache the app shell and queue offline writes. Defer —
  entry cadence is monthly (read statements, type them in) and likely happens at a desk, so offline
  is not yet load-bearing. Add when it bites.
- **Push notifications.** Useful for a monthly-entry reminder. Defer — easy to add when the reminder
  feature is built.
- **Native biometric auth.** Q22 will examine whether WebAuthn / passkeys give us a native-feeling
  biometric flow without needing Capacitor or native code.

## Consequences

- Frontend build produces a standard SPA plus `manifest.json` and a service worker. No
  mobile-specific build pipeline.
- Distribution is "share the URL" — installation is a browser action, not an App Store flow.
- Adopting Capacitor later is purely additive: same Vite output, wrapped in a Capacitor project.
- Authentication and other cross-cutting concerns can target web standards (WebAuthn, etc.) without
  worrying about native compatibility, since the PWA runs in a real browser.
