# Geist (embedded)

`Geist-Regular.ttf` and `Geist-Bold.ttf` are static Latin-subset instances of the
**Geist** typeface (variable font, `wght` axis), instantiated at weight 400 and 700
respectively from the same web font the frontend bundles
(`frontend/dist/assets/geist-latin-wght-normal-*.woff2`).

`Copyright 2024 The Geist Project Authors (https://github.com/vercel/geist-font)`.

Geist is licensed under the **SIL Open Font License 1.1** (OFL) — redistributable and
embeddable, compatible with this project's AGPL-3.0 (ADR-0042). OFL clause 2 requires the
copyright notice and license text to accompany the font files; the full license is bundled
alongside them in [`OFL.txt`](./OFL.txt) (the subset TTFs below carry no name-table metadata
of their own). Upstream source: https://github.com/vercel/geist-font

There is no Reserved Font Name, so the subset instances keep the "Geist" name.

These files are embedded into the backend binary via `go:embed` and subset again into each
generated PDF (ADR-0045).
