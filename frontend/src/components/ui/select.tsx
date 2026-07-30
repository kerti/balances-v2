import * as React from "react";

import { cn } from "@/lib/utils";

// The native-`<select>` counterpart to `Input`, added by #541.
//
// Until now there was no select primitive at all: all 34 callsites hand-rolled
// `h-9 rounded-md border border-input bg-background px-3 text-sm`, which is
// 36px — under the 44px mobile tap floor (INV-PRESENTATION-08 / ADR-0050). #559
// moved the floor into `Button` and `Input` precisely so a new form cannot ship
// under it; `<select>` was the one control class left without a primitive to
// floor, so it kept leaking. Hence `max-md:min-h-11` in the base class here,
// matching `Input` exactly.
//
// `text-base` with `md:text-sm` is also carried over from `Input`, and is load
// bearing rather than cosmetic: iOS Safari zooms the viewport when a form
// control smaller than 16px takes focus, which the old `text-sm` selects did on
// every tap. The sizing otherwise tracks `Input` (`h-8`, `rounded-lg`,
// `bg-transparent`) so a select and a text field sitting next to each other in
// a form row are the same height and shape — previously they were not.
//
// #541 kept the native dropdown arrow on purpose (no `appearance-none` + custom
// chevron), on the reasoning that the platform affordance is the familiar one
// for a non-technical household audience. #572 reverses that, because the
// premise turned out to be untested on the one engine it was about: the floor
// above never actually held on iOS Safari, which draws a **native control with
// its own intrinsic height** and simply ignores `h-8` / `max-md:min-h-11`, so
// on a real iPhone every select in the app sat visibly shorter than the text
// field beside it (width was fine — that part the native control does honour).
// Resetting `appearance` is the only way the height lands, and it takes the
// platform's arrow with it, so the app draws its own.
//
// The tap affordance itself is untouched by any of this: a native `<select>`
// still opens iOS's native sheet or wheel on tap, appearance reset or not —
// what the platform draws *in the page* and what it opens *over* the page are
// separate. So #541's actual goal survives; only the in-page arrow changes.
//
// The chevron arrives as a background image (`--select-chevron` in `index.css`,
// themed there) rather than the lucide `ChevronDown` used everywhere else,
// because a `<select>` can host neither child markup nor a pseudo-element.
// `pr-8` keeps a long option label from running under it.
//
// A caller that genuinely needs a denser control opts out with `max-md:min-h-0`
// (tailwind-merge lets the callsite win), same escape hatch as `Input`.
function Select({ className, ...props }: React.ComponentProps<"select">) {
  return (
    <select
      data-slot="select"
      className={cn(
        "h-8 max-md:min-h-11 w-full min-w-0 appearance-none rounded-lg border border-input bg-transparent bg-[image:var(--select-chevron)] bg-[position:right_0.625rem_center] bg-no-repeat py-1 pl-2.5 pr-8 text-base transition-colors outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 disabled:pointer-events-none disabled:cursor-not-allowed disabled:bg-input/50 disabled:opacity-50 aria-invalid:border-destructive aria-invalid:ring-3 aria-invalid:ring-destructive/20 md:text-sm dark:bg-input/30 dark:disabled:bg-input/80 dark:aria-invalid:border-destructive/50 dark:aria-invalid:ring-destructive/40",
        className,
      )}
      {...props}
    />
  );
}

export { Select };
