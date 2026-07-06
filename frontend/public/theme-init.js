// Applies the persisted theme before first paint so there is no light/dark
// flash. Precedence mirrors resolveBootTheme() in src/theme/index.ts: an
// explicit stored choice (balances.theme), then the OS preference, then dark
// (the app's historical default). Synchronous and loaded ahead of any
// rendering. The React ThemeProvider re-reads the same sources, so its
// initial state matches what this script painted.
//
// Kept as an external file (not inlined in index.html) so the CSP script-src
// can stay 'self' with no inline-script hash to maintain (issue #362).
(function () {
  try {
    var stored = localStorage.getItem("balances.theme");
    var theme =
      stored === "light" || stored === "dark"
        ? stored
        : window.matchMedia && window.matchMedia("(prefers-color-scheme: light)").matches
          ? "light"
          : "dark";
    if (theme === "dark") document.documentElement.classList.add("dark");
  } catch (e) {
    document.documentElement.classList.add("dark");
  }
})();
