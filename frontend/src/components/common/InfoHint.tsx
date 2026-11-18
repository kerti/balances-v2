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

// HintedLabel renders a label with its InfoHint glued to the last word (#931):
// when a long label wraps on a phone, the ⓘ travels with that word instead of
// dropping onto a line of its own.
export function HintedLabel({
  label,
  hint,
  testId,
}: {
  label: string;
  hint?: ReactNode;
  testId?: string;
}) {
  if (!hint) return <>{label}</>;
  const cut = label.lastIndexOf(" ") + 1;
  return (
    <>
      {label.slice(0, cut)}
      <span className="whitespace-nowrap">
        {label.slice(cut)}
        {/* The trigger's tap target is wider than the icon (44px on phones,
            24px from md up), so pull it left until the icon sits 4px off the
            word: -10px (14px padding) and 0 (4px padding). */}
        <InfoHint label={label} testId={testId} className="-my-3 -ml-2.5 md:ml-0">
          {hint}
        </InfoHint>
      </span>
    </>
  );
}
