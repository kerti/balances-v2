// Package version exposes the application's build tag, injected at build time
// via -ldflags -X (Dockerfile's Go build stage), mirroring the SPA's
// VITE_APP_VERSION (issue #75). It's the single server-side source of truth,
// read by /healthz so a deploy can assert the right version rolled out (#355)
// and stamped into the PDF report footer (#414). "dev" for a local,
// non-ldflags build. Safe to bake at build time: identical across every
// environment a single build gets deployed to.
package version

// Version is the build tag (e.g. "v0.8.0-alpha.1"), or "dev" when unset.
var Version = "dev"
