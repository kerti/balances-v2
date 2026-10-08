# Installable PWA: a shell that is never stale, updates the user chooses

Added to the iOS Home Screen, Balances ran as a standalone web clip with no reload control, and a
deploy could stay invisible for a day or more (#716). Two separate causes stacked:

- **iOS resumes, it does not relaunch.** Tapping the icon brings back the suspended webview, still
  on the old JavaScript.
- **The shell had no `Cache-Control`.** `spaHandler` served `index.html` through `http.ServeFile`,
  which sends `Last-Modified` only, so WebKit applied heuristic freshness — about 10% of the file's
  age. A shell built ten days earlier could be reused for a day without revalidating, and that shell
  names the previous build's hashed `/assets/*` chunks.

Standalone mode has no address bar, no pull-to-refresh and no reload button, so neither cause had a
user-facing escape short of killing the app and clearing website data.

## The decision

### 1. Nothing outside `/assets/` is cacheable without revalidation

`spaHandler` sends `Cache-Control: no-cache` on every response outside `/assets/` — the shell, the
client-route fallback, `sw.js`, the manifest, `theme-init.js`, icons — and on the extensionless
`/assets/<client-route>` fallback, which is also the shell. `no-cache` still caches; it revalidates,
costing one 304 per launch. Content-hashed `/assets/*` files are left to ordinary caching: a new
build gets new names. This is INV-SERVING-08 and on its own fixes the HTTP-layer half.

The rule is "outside `/assets/`", not a list of filenames, so a new unhashed root file (a second
icon, a `robots.txt`) is covered without anyone remembering to add it.

### 2. A web app manifest and a minimal service worker (`vite-plugin-pwa`)

The service worker **precaches the built shell and nothing else**. There is no `runtimeCaching`:
`/api/*` always reaches the server, and no financial figure ever lands in a cache where a stale
screen could read as current data. `/api/*` and `/healthz` are on the navigate-fallback denylist so
the installed worker never answers the OAuth redirects (or any server route opened by URL) with the
cached shell.

Offline therefore means the app opens but no data loads. That is deliberate: offline data entry is
the hard part of a PWA (sync, conflict resolution against Household-shared rows) and nothing about
Balances' monthly snapshot cadence needs it.

### 3. Updates wait for a tap (`registerType: 'prompt'`)

A new build installs in the background and waits. Nothing on screen changes until the user taps
**Reload** on the update banner. `autoUpdate` was rejected: Balances is dialog-heavy — snapshot,
transaction, position and import dialogs — and a reload landing mid-entry discards the form to
deliver an update nobody asked for at that moment. This is INV-PRESENTATION-11.

A service worker without this banner would be worse than no service worker: it pins the cached shell
until something activates the waiting worker, re-creating the stale-app bug inside the cache this
ADR adds. The two ship together or not at all.

Registration happens once, in `AppUpdateProvider` above `App`, so it survives the switch between
the pre-auth screens and the signed-in shell, each of which renders its own banner.

### 4. Downloads stay as they are

A sibling app found that inside an installed iOS PWA, a link to a **same-origin path inside the
manifest's scope** replaces the whole app window — `download` and `target` ignored, no way back —
and moved to the share sheet. Balances does not have that problem and does not adopt the fix:
`triggerDownload` saves a fetched `Blob` through a `blob:` URL, which is outside the navigation
scope, so iOS hands it to its file viewer (close, back, Share). Verified on device in the
pre-manifest web clip; re-verified as part of this change's device check. Any future download must
keep going through `triggerDownload`, never an `<a href="/api/...">`.

## Presentation / UX

- **Update banner:** a full-width strip above the page (signed-in shell: above the sticky header;
  pre-auth screens: above the card) — "A new version of Balances is available." with a **Reload**
  button. Secondary tokens, not a warning colour: a waiting update is good news.
- **Offline strip:** same position — "You're offline — Balances needs a connection." Muted tokens,
  not destructive: losing signal is normal. It appears on the browser's `offline` event **or** a
  failed API request, since `navigator.onLine` lies about captive portals and an unreachable server,
  and clears on any API response. A caller cancelling its own request never trips it. This is
  INV-PRESENTATION-12.
- **Manifest:** name and short name "Balances"; `display: standalone`; **no orientation lock** —
  charts and the desktop layout earn landscape. `background_color` and `theme_color` are the
  graphite plate `#14161A` ([[adr-0054]]), which is also the dark theme's background and the app's
  default theme, so the splash matches the first paint. `index.html` carries `theme-color` per OS
  scheme for the browser chrome.
- **Icons:** generated by `make brand` from the brand mark and committed (CI has no rasterizer):
  192/512 `any` from `icon-plated.svg`; 512 maskable and the 180 `apple-touch-icon` from a new
  square-cornered `icon-fullbleed.svg`, because iOS and Android apply their own mask and transparent
  corners render black on iOS. The 0.60 inset keeps the mark inside the maskable 80% safe circle.

## Consequences

- The Vite dev server (and so the Playwright suite) runs no service worker: the plugin's dev build of
  `virtual:pwa-register/react` is a no-op, and vitest aliases the module to a stub.
- `lazyWithReload`'s chunk-error recovery stays: a browser that never installed the worker, or one
  whose cache was evicted, still meets a stale chunk after a deploy.
- Service-worker behaviour is not covered by an automated test — Playwright's WebKit is not iOS
  standalone. The on-device check (install, deploy, banner, Reload, airplane mode, downloads, Google
  sign-in) is a manual step for this change and for any later change to the worker config.
