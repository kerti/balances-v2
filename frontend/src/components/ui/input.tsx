import * as React from "react";

import { cn } from "@/lib/utils";

// The `max-md:min-h-11` in the base class is the app-wide mobile tap floor
// (INV-PRESENTATION-08 / ADR-0050): a text field is an interactive control, and
// the natural `h-8` is 32px. Flooring here rather than per-callsite means a new
// form can't silently ship under the floor — the earlier sweeps (#507–#542)
// added `min-h-11 md:min-h-0` one callsite at a time, which leaked by
// construction. A caller that genuinely needs a denser field can still opt out
// with `max-md:min-h-0` (tailwind-merge lets the callsite win).
//
// The floor did not actually hold for the temporal input types, though (#572).
// A `type="date"` / `type="month"` field is drawn by the *platform*, not by us:
// iOS Safari renders a native control that keeps its own intrinsic width and
// height, so it ignored both `h-8` and `max-md:min-h-11` and refused to shrink
// into its grid column — on a real iPhone the Entry screen's two-column row
// rendered as one continuous bar, the date field taller than its month sibling
// and overflowing the gap between them. Chrome DevTools device mode cannot
// surface this class of bug at all: it emulates the viewport, DPR, touch and UA
// string, but still renders with Blink and Chrome's own form controls, which is
// why the #541 390px pass measured these fields as fitting and cleared them.
// Resetting the appearance hands the box model back to the classes above; the
// picker still opens on tap, because that is the platform's job and not the
// widget's.
//
// The reset is all that's needed: Tailwind's own preflight already normalises
// the WebKit shadow parts *inside* the control (`::-webkit-date-and-time-value`
// gets `min-height: 1lh` + `text-align: inherit`, `::-webkit-datetime-edit`
// becomes an unpadded inline-flex), which is what keeps the value box from
// centring or collapsing once the native chrome is gone. What preflight does not
// do — for any input type — is touch `appearance`, so the outer control kept the
// platform's metrics.
const DATE_LIKE_TYPES = ["date", "month", "week", "time", "datetime-local"];

function Input({ className, type, ...props }: React.ComponentProps<"input">) {
  return (
    <input
      type={type}
      data-slot="input"
      className={cn(
        "h-8 max-md:min-h-11 w-full min-w-0 rounded-lg border border-input bg-transparent px-2.5 py-1 text-base transition-colors outline-none file:inline-flex file:h-6 file:border-0 file:bg-transparent file:text-sm file:font-medium file:text-foreground placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 disabled:pointer-events-none disabled:cursor-not-allowed disabled:bg-input/50 disabled:opacity-50 aria-invalid:border-destructive aria-invalid:ring-3 aria-invalid:ring-destructive/20 md:text-sm dark:bg-input/30 dark:disabled:bg-input/80 dark:aria-invalid:border-destructive/50 dark:aria-invalid:ring-destructive/40",
        type !== undefined && DATE_LIKE_TYPES.includes(type) && "appearance-none",
        className,
      )}
      {...props}
    />
  );
}

export { Input };
