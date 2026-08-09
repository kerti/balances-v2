# Balances — brand: Graphite & Brass

Everything needed to understand and regenerate the identity. Every asset is generated from
`gen.py`, not hand-drawn in a design tool, so the set is reproducible from this directory.

Adopted in **ADR-0054**, replacing the indigo balance-scale identity.

```sh
make brand   # regenerate + fan out to the app + re-export the rasters
```

## Concept

The name says it: **balance**. The identity keeps the two ideas the domain gives it
(`CONTEXT.md`) — net worth tracked as **end-of-month snapshots**, and **assets vs. liabilities** —
but says them once instead of three times:

- **Mark** — a level beam on a fulcrum with a single point balanced above: a snapshot in
  equilibrium. The old mark's hanging stacks / bar-chart double reading is dropped; it was doing
  too much and mushed below ~32px.
- **Wordmark** — the word *becomes* the balance. The `l` is a slender brass post, and its top
  third is cut away on the left, leaving a nib-like diagonal rising to the right.

Design constraints, carried over unchanged:

- Audience is **non-technical** household members → instantly legible, no finance jargon glyphs.
- **Multi-currency** → no currency symbol.
- **Indonesian retail context** (bonds/gold/deposito) → culturally neutral.
- Liabilities are not "bad" (receivables exist) → **no red/green pairing**, no position type coloured
  by whether it is good to own; colour-blind safe throughout. The dashboard's liabilities *line* may
  read warm-red (ADR-0054 §2): that line is purely debt, since receivables fold into assets.

## Asset set (`svg/`)

| File | Use | Canvas | Notes |
|------|-----|--------|-------|
| `icon-plated.svg`  | App icon, PWA, OS       | 256×256 | Full mark on the graphite plate; safe-area padding is intentional. |
| `favicon.svg`      | Browser tab, bookmarks  | 64×64   | **Simplified** mark — heavier beam, wider fulcrum; survives 16px where the full mark thins out. |
| `glyph-light.svg`  | In-UI mark, **light**   | 50×37   | Transparent, cropped tight to the mark. |
| `glyph-dark.svg`   | In-UI mark, **dark**    | 50×37   | Transparent, cropped tight. |
| `wordmark-light.svg` | Wordmark, light bg    | 439×118 | Outlined "balances"; **no mark** — see below. |
| `wordmark-dark.svg`  | Wordmark, dark bg     | 439×118 | Outlined "balances". |

`AppLogo` shows the **wordmark alone**. The identity now lives *in* the word — the tall tapered
brass `l` is the fulcrum post — so setting the mark beside it states the same idea twice. The
**PDF report** shows the wordmark too, drawn as real vector paths from generated data (below). The
mark stands alone only where there is no room for text: favicon and app icon.

### Social preview

`social-card.svg` is the 1280×640 GitHub social-preview card; `make brand` regenerates it and the
upload PNG. It is **generated**, not hand-authored: the pre-rebrand card set the brand name as live
`<text>` in IBM Plex, which silently substituted a system sans whenever the font wasn't visible to
fontconfig at render time. It now embeds the same outlined wordmark the app ships, so there is no
font dependency left. Body copy is deliberately **Helvetica**, so the supporting text reads distinct
from the brand name.

GitHub has no API for the social preview — upload `social-card.png` by hand under repo
**Settings → General → Social preview**.

## Colour tokens

**Brass is the constant brand colour**; the ink/paper roles swap between themes.

| Token | Light | Dark | Role |
|-------|-------|------|------|
| Brass (mark, accent) | `#B08947` | `#C79A4E` | The balanced point; the brand colour. |
| Brass, deep          | `#8A6A30` | `#C79A4E` | `--primary`, focus ring, **any brass that sets type** — see below. |
| Ink                  | `#1C2128` | `#E8E6E1` | Beam + fulcrum; body text. |
| Paper                | `#F7F5F1` | `#14161A` | Page ground. |
| Card                 | `#FFFFFF` | `#1C1F24` | Lifted surface. |
| Muted text           | `#6E675C` | `#9A958B` | Secondary copy. |
| Border               | `#E7E1D8` | `#2A2E35` | Hairlines. |
| Plate                | `#14161A` | `#14161A` | Favicon / app-icon / social ground, both themes. |

**Two brasses, on purpose.** `#B08947` is 3.2:1 on white — fine for a filled shape (which owes
3:1) and wrong for type or a focus ring (which owe 4.5:1 and 3:1 respectively). Anything that sets
type in brass, or draws a focus indicator, uses the deep `#8A6A30`. This is why `--ring` and the
PDF's `accent` are the deep value while the mark's dot is the light one.

## In-app UI tokens

`frontend/src/index.css` carries the full ramp; the values above are its source. Two entries there
depart from this table to clear WCAG AA, and both carry the reasoning inline: `--muted-foreground`
(the brand's `#736C60` is a 4.41:1 near-miss on `--muted`, which the shared `bg-muted/50` CardFooter
actually uses — the same failure #368 fixed once already) and `--ring`.

Two palettes are **not** part of the brand ramp and are deliberately left alone. They are categorical
sets chosen for mutual distinctiveness under colour-vision deficiency, not brand identity —
collapsing them onto the brass/graphite axis would make their series indistinguishable:

- `frontend/src/lib/tagColors.ts` — user-defined position tags
- `frontend/src/components/CategoryStackChartImpl.tsx` — category stack chart

The **dashboard net-worth chart** (`--chart-networth` / `-assets` / `-liabilities` / `-investments`)
is the one place the brand reaches into a chart, and only for one series: net worth is the headline
rather than a category, so it carries the brass, and investments takes the neutral the headline used
to wear. The other three remain a validated categorical triple. `index.css` carries the full
reasoning for each hue — none of it is free choice, and all of it was computed with the dataviz
validator (all-pairs, both modes, against the real card surface) rather than eyeballed.

## Geometry

The mark is drawn on a **64 grid** (`mark_full` / `mark_simple` in `gen.py`); the glyph assets crop
to the mark's tight content bounds + a 3px even pad.

Every element **touches** its neighbour, and that is the load-bearing part of the drawing: the dot
is tangent to the beam (it rests on it rather than hovering) and the fulcrum's apex meets the beam's
underside. A balance whose one contact point floats reads as three unrelated specks the moment it is
scaled down — which is exactly how the first cut of this mark failed at 24px.

## The PDF report's copy

The report draws the wordmark as real vector paths — fpdf has no SVG support and the report has no
image pipeline (ADR-0045). `gen.py` emits `backend/internal/reports/pdf/wordmark_gen.go`: the outline
in font units, y-up, as an SVG path subset (`M`/`L`/`C`/`Z`, absolute, whitespace-separated), which
`logo.go` scales and flips into fpdf's path API. Quadratics are converted to cubics on the way out,
because fpdf takes cubic Béziers.

Two things it is easy to get wrong there, both guarded in code:

- Each colour is traced as **one** path and filled once. The counters in `b`/`a`/`e` are wound
  opposite their outer contours, so filling contours individually would fill the holes in.
- The generator **fails** on an unhandled pen op rather than skipping it. A dropped `qCurveTo`
  degenerates into the following `lineTo` and still produces a plausible-looking path — silence there
  ships a subtly wrong logo.

## Typeface — wordmark

- **Plus Jakarta Sans, weight 700**, tracking −0.02em, outlined to `<path>`.
- The `l` is scaled **×1.4 about the baseline** (foot planted) and its **top third** is cut away on
  the left. Both are exact geometry, not effects: PJS's `l` is a plain rectangle
  (`M60.5 0V757H191.75V0Z`), so the tapered post is emitted as a quad — no clip-path or transform
  survives into the shipped asset.
- **Outlined, not live text** — a logo must render identically on every device, and the shipped SVGs
  contain zero font dependency (no `<text>`, no `font-family`). That matters more here than it did
  for the old wordmark: the app's UI face is **Geist**, not Plus Jakarta Sans, so live text would
  depend on a webfont the app otherwise never loads.
- Licence: **SIL Open Font License (OFL)**. `PlusJakartaSans-var.ttf` is **committed** here so
  `make brand` works from a clean clone — the pre-rebrand `IBMPlexSans-var.ttf` was gitignored, which
  quietly made the generator unreproducible for anyone but the machine that had the font installed.
  The OFL text lives at `frontend/licenses/PlusJakartaSans-OFL.txt` and is folded into the shipped
  `THIRD-PARTY-NOTICES` by `make licenses` (issue #345).

The app's UI typeface is unrelated to the wordmark and is **not** changing: Geist, bundled via
`@fontsource-variable/geist`, guarded by INV-PRESENTATION-09 and embedded in the PDF renderer so an
exported report and the screen it came from are one typeface.

## Usage

Clear space ≥ the diameter of the mark's balanced point. Don't recolour outside the palette, add
gradients or shadows, distort the mark, or set the wordmark in another face. Brass is an accent —
one brass action per view; the neutrals carry the rest.

Prose casing is unaffected by the lowercase logotype: the product is **"Balances"** with a capital B
in all copy; lowercase `balances` is the logo, and a lowercase "balances" in running text means
account balances.
