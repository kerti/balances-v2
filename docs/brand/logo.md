# Balances — logo & brand mark

Everything needed to understand and regenerate the logo. The mark is generated from scripts,
not hand-drawn in a design tool, so it is fully reproducible from this directory.

## Concept

The name says it: **balance**. The mark fuses two meanings drawn straight from the domain
(`CONTEXT.md`): the app tracks **net worth** as **end-of-month snapshots** across a Household's
positions — deliberately *not* a transaction-by-transaction flow.

A **balance scale**, rendered so it reads three ways at once:

- **Fulcrum dot** — a single point = the monthly **snapshot** (a point in time, not a stream).
- **Beam + two hanging stacks** — **assets** (left, indigo, taller) outweigh **liabilities**
  (right, slate, shorter): the scale tips toward positive net worth.
- The stacks double as **bar-chart** bars — net worth observed over time.

Design constraints honoured (see memories / `CONTEXT.md`):
- Audience is **non-technical** household members → instantly legible, no finance jargon glyphs.
- **Multi-currency** → no currency symbol.
- **Indonesian retail context** (bonds/gold/deposito) → culturally neutral.
- Liabilities are not "bad" (receivables exist) → **no red/green** coding; colour-blind safe.

## Asset set (`svg/`)

| File | Use | Canvas | Notes |
|------|-----|--------|-------|
| `icon-plated.svg`  | App icon, PWA, OS, social card | 256×256 | Full mark on navy plate; safe-area padding is intentional. |
| `favicon.svg`      | Browser tab, bookmarks         | 64×64   | **Simplified** mark — one bar per side, no hangers/stacks; survives 16px where the full mark would mush. |
| `glyph-light.svg`  | In-UI mark on **light** theme  | 170×163 | Transparent, cropped tight. |
| `glyph-dark.svg`   | In-UI mark on **dark** theme   | 170×163 | Transparent, cropped tight. |
| `wordmark-light.svg` | Horizontal lockup, light bg  | 284×88  | Glyph + outlined "Balances". |
| `wordmark-dark.svg`  | Horizontal lockup, dark bg   | 284×88  | Glyph + outlined "Balances". |

### Social preview

`social-card.svg` (in this directory, not `svg/`) is the 1280×640 GitHub social-preview card — glyph +
wordmark + value-prop over the navy plate. Regenerate the upload PNG with:

```sh
rsvg-convert -w 2560 -h 1280 docs/brand/social-card.svg -o social-card.png
```

GitHub has no API for the social preview — upload the PNG manually under repo **Settings → General →
Social preview**.

## Colour tokens

The **indigo accent is constant across themes**; only the *ink* (post/beam/hangers) swaps.

| Token | Hex | Role |
|-------|-----|------|
| Accent           | `#6366F1` | Fulcrum dot, top asset bar — the brand colour. |
| Accent mid       | `#818CF8` | Asset bar 2. |
| Accent low       | `#A5B4FC` | Asset bar 3. |
| Plate / dark ink | `#0F172A` | Navy plate; ink on light theme. |
| Light ink        | `#E2E8F0` | Ink on dark theme / on the plate. |
| Liability (light)| `#334155` | Liability bars on light theme / plate. |
| Liability (dark) | `#64748B` | Liability bars on dark theme. |
| Ink-mute (light) | `#94A3B8` | Hangers, light theme. |
| Ink-mute (dark)  | `#475569` | Hangers, dark theme / plate. |
| Preview bg light | `#F8FAFC` | (preview only) |
| Preview bg dark  | `#0B1120` | (preview only) |

## In-app UI tokens

The app's shadcn/Tailwind theme (`frontend/src/index.css`) uses the same **indigo** as the
logo accent for `--primary`, `--ring`, `--chart-1..5`, and the `--sidebar-*` set — Tailwind v4's
oklch indigo-300…700 ramp, anchored on **indigo-500 = `#6366F1`** (the exact logo accent hex).
Neutrals (`--background`, `--card`, `--border`, etc.) stay on shadcn's stone base, unrelated to
brand colour.

| Token role | Light mode | Dark mode | Tailwind step |
|---|---|---|---|
| `--primary` / `--ring` / `--sidebar-primary` / `--sidebar-ring` / `--chart-1` (dark only) | `oklch(0.511 0.262 276.966)` | `oklch(0.673 0.182 276.935)` | indigo-600 / indigo-400 |
| `--chart-1` (light) | `oklch(0.585 0.233 277.117)` | — | indigo-500 (= `#6366F1`) |
| `--chart-2` | `oklch(0.511 0.262 276.966)` | `oklch(0.585 0.233 277.117)` | indigo-600 / indigo-500 |
| `--chart-3` | `oklch(0.457 0.24 277.023)` | `oklch(0.511 0.262 276.966)` | indigo-700 / indigo-600 |
| `--chart-4` | `oklch(0.673 0.182 276.935)` | `oklch(0.785 0.115 274.713)` | indigo-400 / indigo-300 |
| `--chart-5` | `oklch(0.785 0.115 274.713)` | `oklch(0.87 0.065 274.039)` | indigo-300 / indigo-200 |

Dark mode shifts the whole ramp **two** Tailwind steps lighter than light mode (not one) —
indigo's chroma is much higher than a typical shadcn accent (e.g. the cyan ramp this theme
started from), so a one-step shift that reads fine for cyan still reads too dark/moody for
indigo against a dark `--background`. Change the whole ramp by swapping the `oklch(...)` values
for another Tailwind colour's steps, keeping the same role mapping and the two-step light/dark
offset.

Two component-level palettes are **not** part of this brand ramp — they're categorical palettes
picked for mutual distinctiveness (colour-blind safe), not brand identity, and use raw hex rather
than the CSS vars above: `frontend/src/lib/tagColors.ts` (user-defined position tags) and
`frontend/src/components/CategoryStackChartImpl.tsx` (category stack chart). Leave them as-is
when re-theming the brand ramp.

## Geometry

The mark is drawn on a **256 design grid** (`shapes()` in `gen.py`). Tight content bounds are
`x 49–207, y 45–196`; the transparent glyph crops to those bounds + **6px even pad** → a
**170×163** box. The plated icon recentres the mark inside the 256 plate with safe-area padding.

## Typeface — wordmark

- **IBM Plex Sans, weight 700 (Bold), tracking −40** font units, outlined to `<path>`.
- **Outlined, not live text** — a logo must render identically on every device. The shipped
  wordmark SVGs contain **zero font dependency** (no `<text>`, no `font-family`).
- Licence: **SIL Open Font License (OFL)** — free to embed and outline. The wordmark now ships,
  so the OFL text lives at `frontend/licenses/IBMPlexSans-OFL.txt` as attribution (courtesy;
  outlines in a logo don't legally require it) and is folded into the shipped `THIRD-PARTY-NOTICES`
  by `make licenses` (issue #345).

To change the word, weight, or tracking, edit the constants at the top of `outline.py`
(`WEIGHT`, `TRACK`, `FONT_PX`, `TEXT`) and regenerate.

## Regeneration

Prereqs: Python 3, `fonttools`, `curl`, and macOS `qlmanage` (only for PNG previews).

```sh
cd docs/brand

# 1. tooling + font (variable font; both gitignored, not committed)
python3 -m pip install --user fonttools
curl -fsSL -o IBMPlexSans-var.ttf \
  "https://github.com/google/fonts/raw/main/ofl/ibmplexsans/IBMPlexSans%5Bwdth,wght%5D.ttf"

# 2. outline the wordmark text  → writes wordmark_path.json (gitignored, derived)
python3 outline.py

# 3. generate every SVG (writes alongside the scripts)
python3 gen.py

# 4. move the canonical assets into svg/ (the _prev-* files are preview-only, discard)
mv favicon.svg icon-plated.svg glyph-light.svg glyph-dark.svg \
   wordmark-light.svg wordmark-dark.svg svg/
rm -f _prev-*.svg
```

Optional PNG preview of any SVG (macOS): `qlmanage -t -s 512 -o . svg/icon-plated.svg`.
Note `qlmanage` pads thumbnails into a **square** canvas — non-square SVGs look letterboxed in
the PNG, but the `viewBox` itself is tight. Ship the SVGs, not the PNGs.

## Notes / open follow-ups

- The colour ramp is wired into the app theme (see "In-app UI tokens" above); the SVG mark itself
  is not yet installed. To install: favicon `<link rel="icon" href="favicon.svg">`, and a
  theme-switching `<AppLogo>` that picks `glyph-light` / `glyph-dark` (and the matching wordmark)
  from the active theme.
- `IBMPlexSans-var.ttf` and `wordmark_path.json` are **gitignored** (one is downloadable, the
  other is derived). The scripts + final SVGs are the tracked source of truth.
