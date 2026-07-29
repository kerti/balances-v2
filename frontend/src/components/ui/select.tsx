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
// The native dropdown arrow is deliberately kept (no `appearance-none` + custom
// chevron): the platform picker is the familiar affordance for the
// non-technical household audience this app targets, and on phones it opens as
// a native sheet or wheel rather than an in-page menu.
//
// A caller that genuinely needs a denser control opts out with `max-md:min-h-0`
// (tailwind-merge lets the callsite win), same escape hatch as `Input`.
function Select({ className, ...props }: React.ComponentProps<"select">) {
  return (
    <select
      data-slot="select"
      className={cn(
        "h-8 max-md:min-h-11 w-full min-w-0 rounded-lg border border-input bg-transparent px-2.5 py-1 text-base transition-colors outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 disabled:pointer-events-none disabled:cursor-not-allowed disabled:bg-input/50 disabled:opacity-50 aria-invalid:border-destructive aria-invalid:ring-3 aria-invalid:ring-destructive/20 md:text-sm dark:bg-input/30 dark:disabled:bg-input/80 dark:aria-invalid:border-destructive/50 dark:aria-invalid:ring-destructive/40",
        className,
      )}
      {...props}
    />
  );
}

export { Select };
