---
status: supersedes the indigo balance-scale identity (docs/brand/logo.md)
---

# Graphite & Brass — the brand, and where a brand is allowed to reach

The indigo balance-scale identity was default-fintech: shadcn's stone neutrals with Tailwind's
indigo-500 dropped on top, and a mark that tried to read three ways at once (fulcrum dot = snapshot,
beam + hanging stacks = assets outweighing liabilities, stacks doubling as bar-chart bars). It was
legible at 256px and mush below 32. This ADR adopts **Graphite & Brass** — warm graphite neutrals, a
muted brass accent, a simplified equilibrium mark, and a lowercase outlined `balances` wordmark.

The palette and the concept are not the interesting part of this decision. A rebrand's real question
is **how far into the product a brand colour is allowed to reach**, and this one answers it in three
places where the honest answer is "not this far".

## The decision

### 1. Brass is two colours, and which one you use is a contrast question

The brand's accent is `#B08947`. It is **3.2:1 on white**. That is fine for a filled shape — a
graphical object owes 3:1 — and wrong for anything that sets type or draws a focus indicator.

So the palette ships two brasses and the rule that picks between them:

| | value | used for |
|---|---|---|
| Brass | `#B08947` | the mark's balanced point, `--chart-1`, fills |
| Brass, deep | `#8A6A30` | `--primary`, `--ring`, PDF section headings and totals, email CTAs and alt text |

This is why `--ring` is *not* the accent the brand table names: a focus indicator owes 3:1 against
its surround (WCAG 1.4.11) and `#B08947` is 2.96:1 on `--background`. Keeping `ring == primary` also
restores the pairing the pre-rebrand theme had.

The same reasoning moved `--muted-foreground` off the brand's `#736C60`, which is 4.41:1 on
`--muted`. That is not a hypothetical surface: the shared `CardFooter` is `bg-muted/50`, and it is
the exact 4.19:1 near-miss #368 already fixed once on the old palette. `#6E675C` restores the margin.

The general rule: **a brand table names hues, not roles.** Every place a hue becomes type,
a border, or a focus ring, it has to re-clear the bar for that role, and the brand table is not
evidence that it does.

### 2. Three palettes are not brand, and the rebrand does not touch them

`tagColors.ts`, `CategoryStackChartImpl.tsx`, and the four dashboard net-worth chart tokens
(`--chart-networth` / `-assets` / `-liabilities` / `-investments`) are **categorical** sets chosen
for mutual distinctiveness under colour-vision deficiency. They are not the brand ramp and were
deliberately left alone: pulled onto the brass/graphite axis, their series stop being tellable apart,
which for the net-worth chart would mean three of four components collapsing into one another.

They were re-validated rather than assumed, because the rebrand *did* move the ground under them —
light `--card` goes from `#F7F5F1` to pure white. All four still clear the 3:1 graphical floor.

This is the counterpart to the rule above: a brand reaches every surface where colour carries
*identity*, and stops at every surface where colour carries *data*.

### 3. The wordmark's typeface is not the app's typeface

The wordmark is **Plus Jakarta Sans 700**, outlined to paths. The app's UI face stays **Geist** and
was never in scope, despite the rebrand proposal describing the change as "Plus Jakarta Sans
throughout, replacing IBM Plex Sans" — IBM Plex was only ever the outlined wordmark. The app has run
on Geist since #565.

Keeping them separate is the cheap option and also the correct one. Geist is:

- pinned by **INV-PRESENTATION-09** and `typography.spec.ts`;
- **embedded in the PDF renderer**, deliberately, so an exported report and the screen it came from
  are one typeface;
- the reason every text-width assertion in the suite is platform-stable (`system-ui` resolved to a
  different physical font per platform, a ~12% width swing between local and CI).

A logo face distinct from the UI face is normal, and is what this repo already did.

Because the wordmark ships **outlined**, Plus Jakarta Sans is not a runtime dependency at all — no
npm package, no webfont, no `@font-face`. The TTF lives in `docs/brand/` for the generator only.

### 4. The mark's elements touch

The staged mark drew a dot floating above a beam, and a fulcrum whose apex stopped 2px short of the
beam's underside. Rendered at 24px and 16px it is three unrelated specks — the same failure mode the
old mark had, arrived at from the opposite direction (too sparse rather than too busy).

The shipped mark closes both gaps: the dot is **tangent** to the beam, the fulcrum's apex **meets**
it, and the whole drawing fills its box rather than floating in the design grid's slack. A separate
heavier cut (`mark_simple`) is the favicon, the same discipline the pre-rebrand brand applied to its
own small size.

### 5. The wordmark stands alone; the mark stands alone

`AppLogo` shows the **word only**. The identity now lives in the word — the `l` is a tall brass post
with its top third cut away, a nib-like diagonal rising to the right — so setting the mark beside it
states the same idea twice.

The mark stands alone wherever there is no room for text: favicon and app icon.

The **PDF report shows the wordmark**, drawn as real vector paths. The pre-rebrand report set the
brand name as live Geist Bold text beside the mark, because the wordmark was a custom-typeface asset
it had no way to reproduce — fpdf has no SVG support and the report has no image pipeline
(ADR-0045). That approximation is gone: the outline is now **generated into the Go package** by the
same `gen.py` that emits the SVGs (quadratics converted to cubics for fpdf's path API), so the
report's wordmark and the shipped asset cannot drift. Each colour is filled as one path, so the
counters in `b`/`a`/`e` stay hollow under the nonzero winding rule.

The email header is the one place the wordmark ships as a raster, because mail clients strip
`@font-face` and cannot draw vectors — `make brand` re-exports it.

### 6. One generator, one make target

`gen.py` now emits the whole set — marks, favicon, app icon, both wordmarks, and the social card —
replacing the `gen.py` + `outline.py` + `wordmark_path.json` trio (the wordmark needs per-glyph fills
for the brass `l`, which a single pre-outlined path blob could not express).

`make brand` regenerates and fans the results out to the three places the app reads them from. That
is new, and it is the fix for a real drift: `docs/brand/svg/` and `frontend/src/assets/brand/` held
separately-hand-copied versions of the same files.

The social card is now **generated** too. It used to set the brand name as live `<text>` in IBM Plex
with a documented warning that the name silently substitutes a system sans if the font isn't visible
to fontconfig at render time. Embedding the outlined wordmark deletes the failure mode rather than
documenting it.

`gen.py` also emits `backend/internal/reports/pdf/wordmark_gen.go`, so the report's copy of the
wordmark is generated rather than hand-ported. Nothing about the brand is hand-maintained in two
places any more.

## Consequences

- Every themed surface changes at once. This is a pre-alpha project with no real users; there is no
  migration and nothing to stage.
- `--card` becomes pure white in light mode, where it previously sat at exactly `--background`. Cards
  used to separate from the page by border alone; they now sit on warm paper. This is the largest
  *structural* visual change in the rebrand and is intentional.
- Dark `--border` becomes a solid graphite instead of a white alpha wash, which on a warm surface
  greyed the hue out of every edge.
- The email templates get a contrast fix that rides along rather than being part of the brand: the
  footer was slate-400 on white, 2.6:1, and had never passed.
- IBM Plex Sans leaves the repo (font, OFL text, generator); Plus Jakarta Sans arrives.
  `THIRD-PARTY-NOTICES` is regenerated.
- `social-card.png` still has to be uploaded by hand — GitHub has no API for the social preview.

## Considered alternatives

- **Ship the staged package verbatim.** Rejected on the four counts above: it would have landed a
  failing `axe` gate (`--muted-foreground`), a sub-3:1 focus ring, a mark that dies at 24px, and a
  UI-wide font swap premised on a typeface the app does not use.
- **Switch the UI to Plus Jakarta Sans as well.** Rejected as a much larger, separable change: it
  rewrites INV-PRESENTATION-09, swaps the PDF's embedded fonts, and re-baselines every
  width-sensitive assertion. Nothing about the brand requires it.
- **Keep a mark + wordmark lockup in-app.** Rejected as redundant once the `l` became the post.
- **Keep the wordmark as live text** (the staged package shipped a `Wordmark.tsx` doing the `l`
  transform in CSS `em` units). Rejected: it reintroduces the #565 webfont-race class of bug — with
  the face unloaded, the `scaleY` and the offsets land on fallback metrics and the logo visibly
  breaks — for a face the app otherwise never loads.
