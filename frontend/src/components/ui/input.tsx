import * as React from "react";

import { cn } from "@/lib/utils";

// The `max-md:min-h-11` in the base class is the app-wide mobile tap floor
// (INV-PRESENTATION-08 / ADR-0050): a text field is an interactive control, and
// the natural `h-8` is 32px. Flooring here rather than per-callsite means a new
// form can't silently ship under the floor — the earlier sweeps (#507–#542)
// added `min-h-11 md:min-h-0` one callsite at a time, which leaked by
// construction. A caller that genuinely needs a denser field can still opt out
// with `max-md:min-h-0` (tailwind-merge lets the callsite win).
function Input({ className, type, ...props }: React.ComponentProps<"input">) {
  return (
    <input
      type={type}
      data-slot="input"
      className={cn(
        "h-8 max-md:min-h-11 w-full min-w-0 rounded-lg border border-input bg-transparent px-2.5 py-1 text-base transition-colors outline-none file:inline-flex file:h-6 file:border-0 file:bg-transparent file:text-sm file:font-medium file:text-foreground placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 disabled:pointer-events-none disabled:cursor-not-allowed disabled:bg-input/50 disabled:opacity-50 aria-invalid:border-destructive aria-invalid:ring-3 aria-invalid:ring-destructive/20 md:text-sm dark:bg-input/30 dark:disabled:bg-input/80 dark:aria-invalid:border-destructive/50 dark:aria-invalid:ring-destructive/40",
        className,
      )}
      {...props}
    />
  );
}

export { Input };
