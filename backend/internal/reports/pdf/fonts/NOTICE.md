# Geist (embedded)

`Geist-Regular.ttf` and `Geist-Bold.ttf` are static Latin-subset instances of the
**Geist** typeface (variable font, `wght` axis), instantiated at weight 400 and 700
respectively from the same web font the frontend bundles
(`frontend/dist/assets/geist-latin-wght-normal-*.woff2`).

Geist is licensed under the **SIL Open Font License 1.1** (OFL) — redistributable and
embeddable, compatible with this project's AGPL-3.0 (ADR-0042). Source and full license:
https://github.com/vercel/geist-font

These files are embedded into the backend binary via `go:embed` and subset again into each
generated PDF (ADR-0045).
