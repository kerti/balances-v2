#!/usr/bin/env python3
"""Generate the Balances brand asset set — Graphite & Brass (ADR-0054).

One script produces every shipped asset: the equilibrium mark (glyphs, favicon,
app icon) and the outlined "balances" wordmark. It replaces the old gen.py +
outline.py + wordmark_path.json trio; the wordmark now needs per-glyph fills
(the brass `l`), which a single pre-outlined path blob could not express.

Run via `make brand`, which also copies the results to their app destinations
and re-exports the rasters. Requires fontTools; PlusJakartaSans-var.ttf ships
alongside this file.
"""
import pathlib
import shutil
import subprocess

OUT = pathlib.Path(__file__).parent

# ---------------------------------------------------------------- palette ---
# Brass is the constant brand colour; ink/paper swap between themes.
BRASS_LIGHT = "#B08947"
BRASS_DARK = "#C79A4E"
INK_LIGHT = "#1C2128"  # graphite
INK_DARK = "#E8E6E1"  # warm off-white
PLATE = "#14161A"  # graphite black, favicon/app-icon/social plate

LIGHT = dict(ink=INK_LIGHT, brass=BRASS_LIGHT)
DARK = dict(ink=INK_DARK, brass=BRASS_DARK)
ON_PLATE = dict(ink=INK_DARK, brass=BRASS_DARK)

# ------------------------------------------------------------------- mark ---
# A level beam on a fulcrum with a single point balanced above: a snapshot in
# equilibrium. Drawn on a 64 grid.
#
# Every element touches its neighbour on purpose. The dot is tangent to the
# beam (it rests on it rather than hovering) and the fulcrum's apex meets the
# beam's underside — a balance drawing whose one contact point floats reads as
# three unrelated specks the moment it is scaled down.
#
# `full` is the in-UI/app-icon mark. `simple` is the small-size cut: same
# construction, heavier strokes and a wider fulcrum so it survives 16px, the
# same discipline the pre-rebrand brand applied to its own favicon.

MARK_BOX = 64


def mark_full(ink, brass):
    return (
        f'<circle cx="32" cy="24" r="7" fill="{brass}"/>'
        f'<rect x="10" y="31" width="44" height="6" rx="3" fill="{ink}"/>'
        f'<path d="M32 37 L23 48 L41 48 Z" fill="{ink}"/>'
    )


def mark_simple(ink, brass):
    return (
        f'<circle cx="32" cy="22" r="8" fill="{brass}"/>'
        f'<rect x="9" y="30" width="46" height="7" rx="3.5" fill="{ink}"/>'
        f'<path d="M32 37 L20 49 L44 49 Z" fill="{ink}"/>'
    )


# Tight content bounds of each mark on the 64 grid, + an even pad. The glyph
# assets crop to these so the mark fills its box rather than floating in the
# design grid's slack.
FULL_BOUNDS = (10, 17, 54, 48)  # circle top y=24-7; fulcrum base y=48
SIMPLE_BOUNDS = (9, 14, 55, 49)
PAD = 3


def cropped(shapes, bounds, ink, brass):
    x0, y0, x1, y1 = bounds
    w, h = (x1 - x0) + 2 * PAD, (y1 - y0) + 2 * PAD
    body = f'<g transform="translate({PAD - x0},{PAD - y0})">{shapes(ink, brass)}</g>'
    return w, h, body


# --------------------------------------------------------------- wordmark ---
# Full lowercase "balances", Plus Jakarta Sans 700, outlined to <path> so the
# shipped asset carries zero font dependency (the same rule the IBM Plex
# wordmark followed). Two moves make it the logo, both on the `l`:
#
#   1. it is a post — scaled x1.4 about the baseline, foot planted;
#   2. its top third is cut away on the left, leaving a nib-like diagonal
#      rising to the right.
#
# Both are exact geometry rather than effects: PJS's `l` is a plain rectangle
# (`M60.5 0V757H191.75V0Z`), so the tall tapered stem is emitted as a quad and
# no clip-path or transform survives into the asset.

TEXT = "balances"
WEIGHT = 700
FONT_PX = 100  # design size; assets are scale-free vectors regardless
TRACK_EM = -0.02  # tracking, em
L_SCALE = 1.4  # post height, about the baseline
L_TAPER = 1 / 3  # fraction of the post's height the cut spans
WM_PAD = 6


def _font():
    from fontTools.ttLib import TTFont
    from fontTools.varLib.instancer import instantiateVariableFont

    f = TTFont(OUT / "PlusJakartaSans-var.ttf")
    instantiateVariableFont(f, {"wght": WEIGHT}, inplace=True)
    return f


def wordmark(ink, brass):
    """Outlined wordmark. Returns (width, height, body) in px at FONT_PX."""
    from fontTools.pens.svgPathPen import SVGPathPen
    from fontTools.pens.boundsPen import BoundsPen

    f = _font()
    upem = f["head"].unitsPerEm
    cmap, hmtx, gs = f.getBestCmap(), f["hmtx"], f.getGlyphSet()
    s = FONT_PX / upem
    track = TRACK_EM * FONT_PX

    x, parts, top = 0.0, [], 0.0
    for ch in TEXT:
        gname = cmap[ord(ch)]
        adv = hmtx[gname][0] * s
        if ch == "l":
            b = BoundsPen(gs)
            gs[gname].draw(b)
            lx0, ly0, lx1, ly1 = b.bounds
            ly1 *= L_SCALE
            cut = ly1 - (ly1 - ly0) * L_TAPER
            # Quad in font units, y-up; emitted through the same flip as the
            # other glyphs. Right edge full height, left edge cut back.
            d = f"M{lx0} {ly0}L{lx1} {ly0}L{lx1} {ly1:.2f}L{lx0} {cut:.2f}Z"
            parts.append(
                f'<g transform="translate({x:.2f},0) scale({s:.6f},{-s:.6f})" '
                f'fill="{brass}"><path d="{d}"/></g>'
            )
            top = max(top, ly1 * s)
        else:
            pen = SVGPathPen(gs)
            gs[gname].draw(pen)
            b = BoundsPen(gs)
            gs[gname].draw(b)
            if b.bounds:
                top = max(top, b.bounds[3] * s)
            parts.append(
                f'<g transform="translate({x:.2f},0) scale({s:.6f},{-s:.6f})" '
                f'fill="{ink}"><path d="{pen.getCommands()}"/></g>'
            )
        x += adv + track

    width = x - track
    w = round(width + 2 * WM_PAD, 2)
    h = round(top + 2 * WM_PAD, 2)
    body = f'<g transform="translate({WM_PAD},{h - WM_PAD:.2f})">{"".join(parts)}</g>'
    return w, h, body


# ------------------------------------------------------------------ emit ----
def svg(w, h, body, label):
    return (
        f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {w} {h}" '
        f'width="{w}" height="{h}" role="img" aria-label="{label}">\n'
        f"  {body}\n</svg>\n"
    )


def plated(size, radius, shapes, bounds, inset):
    """Mark centred on the graphite plate, with deliberate safe-area padding."""
    w, h, body = cropped(shapes, bounds, **ON_PLATE)
    scale = (size * inset) / max(w, h)
    tx, ty = (size - w * scale) / 2, (size - h * scale) / 2
    return (
        f'<rect width="{size}" height="{size}" rx="{radius}" fill="{PLATE}"/>\n'
        f'  <g transform="translate({tx:.2f},{ty:.2f}) scale({scale:.5f})">{body}</g>'
    )


# ------------------------------------------------- wordmark, for the PDF ----
# The report renders the wordmark as real vector paths too (fpdf has no SVG
# support and the report has no image pipeline — ADR-0045). Rather than hand-
# porting it the way the mark is ported, the outline is generated into the Go
# package, so the report's wordmark and the shipped SVG cannot drift.
#
# Quadratics are converted to cubics because fpdf's path API takes cubic Béziers;
# the emitted token stream is an SVG path subset (M/L/C/Z, absolute, whitespace-
# separated) in font units, y-up, which the Go side scales and flips.

PDF_GO = pathlib.Path("backend/internal/reports/pdf/wordmark_gen.go")


def wordmark_paths():
    """(ink tokens, brass tokens, advance, top, pad) in font units."""
    from fontTools.pens.recordingPen import RecordingPen
    from fontTools.pens.qu2cuPen import Qu2CuPen
    from fontTools.pens.boundsPen import BoundsPen

    f = _font()
    upem = f["head"].unitsPerEm
    cmap, hmtx, gs = f.getBestCmap(), f["hmtx"], f.getGlyphSet()
    track = TRACK_EM * upem
    pad = WM_PAD * upem / FONT_PX

    ink, brass, x, top = [], [], 0.0, 0.0

    def emit(out, rec, dx):
        for op, pts in rec.value:
            if op == "moveTo":
                out.append(f"M {pts[0][0] + dx:g} {pts[0][1]:g}")
            elif op == "lineTo":
                out.append(f"L {pts[0][0] + dx:g} {pts[0][1]:g}")
            elif op == "curveTo":
                out.append("C " + " ".join(f"{p[0] + dx:g} {p[1]:g}" for p in pts))
            elif op == "closePath":
                out.append("Z")
            else:
                # Never skip an op quietly: a dropped qCurveTo still produces a
                # plausible-looking path (the curve degenerates to the following
                # lineTo), so silence here ships a subtly wrong logo.
                raise SystemExit(f"wordmark: unhandled pen op {op!r}")

    for ch in TEXT:
        gname = cmap[ord(ch)]
        adv = hmtx[gname][0]
        if ch == "l":
            b = BoundsPen(gs)
            gs[gname].draw(b)
            lx0, ly0, lx1, ly1 = b.bounds
            ly1 *= L_SCALE
            cut = ly1 - (ly1 - ly0) * L_TAPER
            brass.append(
                f"M {lx0 + x:g} {ly0:g} L {lx1 + x:g} {ly0:g} "
                f"L {lx1 + x:g} {ly1:g} L {lx0 + x:g} {cut:g} Z"
            )
            top = max(top, ly1)
        else:
            rec = RecordingPen()
            gs[gname].draw(Qu2CuPen(rec, 0.1, all_cubic=True))
            emit(ink, rec, x)
            b = BoundsPen(gs)
            gs[gname].draw(b)
            if b.bounds:
                top = max(top, b.bounds[3])
        x += adv + track

    return " ".join(ink), " ".join(brass), x - track, top, pad


def go_wordmark():
    ink, brass, adv, top, pad = wordmark_paths()
    return f'''// Code generated by docs/brand/gen.py — DO NOT EDIT. Run `make brand`.

package pdf

// The outlined "balances" wordmark, in font units, y-up, as an SVG path subset
// (M/L/C/Z, absolute, whitespace-separated). Two paths because the tapered `l`
// post is brass and the rest is ink; each is filled as one path so the counters
// in b/a/e stay hollow under the nonzero winding rule.
const (
	wordmarkAdvance = {adv:g} // width of the word itself
	wordmarkTop     = {top:g} // baseline to the top of the post
	wordmarkPad     = {pad:g} // even pad, matching the SVG asset's box

	wordmarkInkPath   = "{ink}"
	wordmarkBrassPath = "{brass}"
)
'''


# --------------------------------------------------------- social preview ---
# The 1280x640 GitHub social-preview card. Generated rather than hand-authored
# so the wordmark on it is the same outlined asset the app ships: the old card
# set the brand name as live <text> in IBM Plex, which meant the name silently
# substituted a system sans whenever the font wasn't visible to fontconfig at
# render time. Body copy stays live Helvetica on purpose — the brand name reads
# distinct from the supporting text.

CARD_COPY = (
    "Track your household's net worth",
    "without itemising a single transaction.",
    "Enter your balances once a month. That's it.",
)
CARD_CHIPS = ("Household-first", "Snapshot-first", "Self-hosted", "Multi-currency")


def social_card():
    w, h, _ = wordmark(**ON_PLATE)
    _, _, wm_body = wordmark(**ON_PLATE)
    tile, tile_y = 132, 64

    # Scale the wordmark so its baseline lands on the mark tile's bottom edge;
    # the tapered post then rises to just above the tile's top.
    wm_h = 150.0
    wm_scale = wm_h / h
    baseline_frac = (h - WM_PAD) / h
    wm_y = (tile_y + tile) - baseline_frac * wm_h

    mw, mh, mark_body = cropped(mark_full, FULL_BOUNDS, **ON_PLATE)
    msc = (tile * 0.74) / max(mw, mh)

    # Chip widths are estimated from the label length rather than measured —
    # Helvetica at 24px averages ~0.55em per character — so the run is checked
    # against the canvas below instead of trusted.
    chips, cx = [], 100
    for label in CARD_CHIPS:
        cw = 24 * len(label) * 0.55 + 56
        chips.append(
            f'<rect x="{cx:.0f}" y="556" width="{cw:.0f}" height="52" rx="26" '
            f'fill="none" stroke="#2A2E35" stroke-width="1.5"/>'
            f'<text x="{cx + cw / 2:.0f}" y="590" text-anchor="middle">{label}</text>'
        )
        cx += cw + 16
    if cx - 16 > 1280 - 100:
        raise SystemExit(f"social card: chip run overflows the canvas ({cx - 16:.0f} > 1180)")

    esc = lambda s: s.replace("&", "&amp;").replace("<", "&lt;")  # noqa: E731
    return f"""<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1280 640" width="1280" height="640">
  <defs>
    <radialGradient id="glow" cx="50%" cy="50%" r="50%">
      <stop offset="0%" stop-color="{BRASS_DARK}" stop-opacity="0.20"/>
      <stop offset="70%" stop-color="{BRASS_DARK}" stop-opacity="0"/>
    </radialGradient>
  </defs>
  <rect width="1280" height="640" fill="{PLATE}"/>
  <circle cx="1180" cy="620" r="360" fill="url(#glow)"/>
  <g transform="translate(100,{tile_y})">
    <rect width="{tile}" height="{tile}" rx="29" fill="#1C1F24"/>
    <g transform="translate({(tile - mw * msc) / 2:.2f},{(tile - mh * msc) / 2:.2f}) scale({msc:.5f})">{mark_body}</g>
  </g>
  <g transform="translate(256,{wm_y:.2f}) scale({wm_scale:.5f})">{wm_body}</g>
  <g font-family="Helvetica,Arial,sans-serif" font-weight="700" font-size="52">
    <text x="100" y="316" fill="{INK_DARK}">{esc(CARD_COPY[0])}</text>
    <text x="100" y="384" fill="{BRASS_DARK}">{esc(CARD_COPY[1])}</text>
  </g>
  <text x="100" y="466" font-family="Helvetica,Arial,sans-serif" font-size="30" fill="#9A958B">{esc(CARD_COPY[2])}</text>
  <g font-family="Helvetica,Arial,sans-serif" font-weight="500" font-size="24" fill="#9A958B">
    {"".join(chips)}
  </g>
</svg>
"""


def main():
    files = {}

    # transparent in-UI marks, cropped tight
    for name, pal in (("light", LIGHT), ("dark", DARK)):
        w, h, body = cropped(mark_full, FULL_BOUNDS, **pal)
        files[f"svg/glyph-{name}.svg"] = svg(w, h, body, "Balances mark")

    # favicon: the simplified cut, so it holds at 16px
    files["svg/favicon.svg"] = svg(
        64, 64, plated(64, 14, mark_simple, SIMPLE_BOUNDS, 0.66), "Balances"
    )
    # app icon: the full mark, which has the room at 256
    files["svg/icon-plated.svg"] = svg(
        256, 256, plated(256, 56, mark_full, FULL_BOUNDS, 0.60), "Balances app icon"
    )

    # outlined wordmarks
    for name, pal in (("light", LIGHT), ("dark", DARK)):
        w, h, body = wordmark(**pal)
        files[f"svg/wordmark-{name}.svg"] = svg(w, h, body, "Balances")

    files["social-card.svg"] = social_card()

    for name, data in files.items():
        p = OUT / name
        p.parent.mkdir(parents=True, exist_ok=True)
        p.write_text(data)

    go = OUT.parent.parent / PDF_GO
    go.write_text(go_wordmark())
    # gofmt rather than hand-aligning: the const block's comment alignment
    # depends on the emitted numbers' widths, which change with the geometry, so
    # a generator that formats by hand goes stale the first time anything moves
    # and fails the backend lint gate.
    if shutil.which("gofmt"):
        subprocess.run(["gofmt", "-w", str(go)], check=True)
    else:
        print("⚠ gofmt not found — wordmark_gen.go may fail the lint gate")

    print(f"wrote {len(files)} svgs + {go.relative_to(OUT.parent.parent)}")


if __name__ == "__main__":
    main()
