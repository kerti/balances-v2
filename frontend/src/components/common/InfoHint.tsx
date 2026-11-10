import { useState, type ReactNode } from "react";
import { Info } from "lucide-react";

import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { cn } from "@/lib/utils";

// InfoHint is the one way to attach an explanation to a label: a keyboard-
// focusable button that opens a popover on tap, mouse hover or focus. It
// replaces the "ⓘ" glyph and explanatory `title` tooltips, which touch screens
// never show. `title` stays only for plain truncation tooltips.
//
// The trigger is a 44px target on phones (ADR-0050) and a compact icon from
// md up. Pass `trigger` to make something other than the Info icon the thing
// that is tapped (the risk badge); `label` is the button's accessible name.
export function InfoHint({
  label,
  children,
  trigger,
  testId = "info-hint",
  className,
}: {
  label: string;
  children: ReactNode;
  trigger?: ReactNode;
  testId?: string;
  className?: string;
}) {
  const [open, setOpen] = useState(false);
  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type="button"
          aria-label={label}
          data-testid={testId}
          className={cn(
            "inline-flex min-h-11 min-w-11 items-center justify-center rounded-full align-middle text-muted-foreground outline-none hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring md:min-h-6 md:min-w-6",
            className,
          )}
          onFocus={() => setOpen(true)}
          onPointerEnter={(e) => e.pointerType === "mouse" && setOpen(true)}
          onPointerLeave={(e) => e.pointerType === "mouse" && setOpen(false)}
          // Focus already opened it, so a plain toggle would close it again on
          // the same tap; stopPropagation keeps a tap inside a clickable row
          // from also navigating.
          onClick={(e) => {
            e.preventDefault();
            e.stopPropagation();
            setOpen(true);
          }}
        >
          {trigger ?? <Info className="size-4" aria-hidden="true" />}
        </button>
      </PopoverTrigger>
      <PopoverContent
        align="start"
        className="w-64 text-sm"
        data-testid={`${testId}-content`}
        // Keep focus on the trigger: moving it into the popover would blur the
        // trigger, and handing it back on close would re-open on focus.
        onOpenAutoFocus={(e) => e.preventDefault()}
        onCloseAutoFocus={(e) => e.preventDefault()}
      >
        {children}
      </PopoverContent>
    </Popover>
  );
}
