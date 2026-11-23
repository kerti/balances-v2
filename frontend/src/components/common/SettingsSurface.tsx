import type { ReactNode } from "react";
import { Card } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { cn } from "@/lib/utils";

// Settings speaks two visual languages, and the split is policy rather than
// accident (#563): **a settings table for scalar preferences, panel cards for
// actions and flows.**
//
// Profile and Household are seven scalar values and one toggle, so they are
// rows of one `SettingsTable` each — thirteen separate cards made thirteen
// scroll stops out of what is mostly a preferences list, and gave each scalar a
// card title that its field `Label` then repeated verbatim ("Household name" /
// "Household name"). Members and Data hold the flows: Members is a table of
// people and pending invites with Invite as its last row (#951), Restore is
// file-pick → preview → acknowledge → type ERASE → commit, and Erase has its
// own confirm gate. Backup, Restore and Erase don't survive being squeezed
// into a row, so each is a `SettingsPanel` — stacked prose and controls, with
// its own heading. Rebuild reports is not a data flow: it is a plain row in
// its own Reports card.
//
// A new setting therefore lands as a `SettingsRow`; a new action or wizard
// lands as a `SettingsPanel`. Both group into one card per section, so the
// screen reads as four blocks rather than thirteen.

// SettingsTable holds the scalar-preference rows of one section.
export function SettingsTable({ children }: { children: ReactNode }) {
  return <Card className="gap-0 divide-y divide-foreground/10 py-0">{children}</Card>;
}

// One row of that table: the setting's name and description stacked in the
// first cell, its control in the second. Deliberately two columns and not
// three — these descriptions run 150–300 characters (inflation is ~300), so a
// dedicated description column would produce wildly unequal row heights and a
// ragged control column.
//
// Same markup at both widths (`flex-col` below 768px, two columns above) — no
// `useIsMobile` renderer split, because nothing here needs different content on
// a phone, only different flow direction.
//
// The control column is fixed-width and left-aligned inside itself so every
// control shares a left edge and every Save shares a right edge. Right-aligning
// the column instead would line the buttons up but leave the control left edges
// ragged (a 96px currency box against a full-width select), which reads
// unfinished in a table. `w-72` is what the old per-card desktop layout already
// came to: a `w-56` input plus a natural-width Save plus the gap.
//
// `htmlFor` is how the row name earns its keep as the field's *actual* label
// rather than a heading sitting next to a nameless textbox: pass it and the
// name renders as `<label for>`, which is why the per-field `Label` could be
// dropped instead of hidden. Currency and inflation are labelled by their row
// name too (#951); only the multi-currency checkbox omits `htmlFor` and
// supplies its own `Label` in the control cell.
export function SettingsRow({
  name,
  description,
  htmlFor,
  required,
  wide,
  children,
}: {
  name: ReactNode;
  description?: ReactNode;
  htmlFor?: string;
  // The row name is the field's label, so it carries the required marker (#953).
  required?: boolean;
  // A field plus an action (Invite: email + Send) needs more than the w-72
  // column, or the address is cut off mid-placeholder.
  wide?: boolean;
  children: ReactNode;
}) {
  return (
    <div
      data-slot="settings-row"
      data-wide={wide || undefined}
      className="flex flex-col gap-2 p-4 md:flex-row md:items-start md:gap-6"
    >
      <div className="space-y-1 md:flex-1">
        {htmlFor ? (
          <Label required={required} htmlFor={htmlFor}>
            {name}
          </Label>
        ) : (
          <p className="text-sm leading-none font-medium">{name}</p>
        )}
        {description && <p className="text-sm text-muted-foreground">{description}</p>}
      </div>
      <div className={cn("w-full space-y-2 md:shrink-0", wide ? "md:w-96" : "md:w-72")}>
        {children}
      </div>
    </div>
  );
}

// SettingsControlRow is the one labelled-control shape on this screen: the
// label/control column grows to fill the space it is given and an optional
// action keeps its natural width, which lands it flush against the right edge.
// The button deliberately does not grow — a 250px "Save" on a phone is worse
// than the gap it would close. Used by the table rows and by the Invite panel,
// whose Send sits to the right of the email field for the same reason.
//
// On a settings row the Save renders always and disables until the row is
// dirty rather than appearing on first keystroke: appearing beside a `flex-1`
// input would reflow that input mid-type, and the permanently-visible button is
// also the answer to "does this row autosave?" — adjacent rows now mix both
// behaviours (language and theme are buttonless per ADR-0032).
export function SettingsControlRow({
  children,
  action,
}: {
  children: ReactNode;
  action?: ReactNode;
}) {
  return (
    <div data-slot="settings-control-row" className="flex items-end gap-2">
      <div className="flex-1 space-y-1">{children}</div>
      {action}
    </div>
  );
}

// SettingsPanelGroup is the flow-side counterpart of SettingsTable: one card
// per section, its panels separated by the same hairline the table rows use, so
// Data reads as one block of three steps rather than three competing cards.
export function SettingsPanelGroup({ children }: { children: ReactNode }) {
  return <Card className="gap-0 divide-y divide-foreground/10 py-0">{children}</Card>;
}

// One flow inside that group. `tone="destructive"` tints the whole panel rather
// than just outlining it: Erase is the one irreversible action on this screen
// and it sat visually level with "Download backup". The tint is a wash
// (`bg-destructive/10`) and not a solid red fill on purpose — body copy stays on
// the normal foreground colour and keeps its contrast ratio, while the band
// still reads as a different kind of place at a glance. It bleeds to the card's
// edges because the group card drops its own padding, so the panel's `p-4` is
// the only inset.
export function SettingsPanel({
  title,
  description,
  tone = "default",
  children,
}: {
  title: ReactNode;
  description?: ReactNode;
  tone?: "default" | "destructive";
  children: ReactNode;
}) {
  return (
    <section
      data-slot="settings-panel"
      data-tone={tone}
      className={cn("space-y-4 p-4", tone === "destructive" && "bg-destructive/10")}
    >
      <div className="space-y-1">
        <h3
          className={cn(
            "font-heading text-base leading-snug font-medium",
            tone === "destructive" && "text-destructive",
          )}
        >
          {title}
        </h3>
        {description && <p className="text-sm text-muted-foreground">{description}</p>}
      </div>
      {children}
    </section>
  );
}

// PanelActions lays out a panel's action buttons: stacked and full-width on a
// phone, inline at their natural widths from 768px up. A phone-width "Delete
// this household…" or "Download backup" reading as a small chip in a wide empty
// row is the thing this fixes — these are the panel's point, not an aside.
// A trailing hint (`note`) drops below the buttons rather than beside them.
export function PanelActions({ children, note }: { children: ReactNode; note?: ReactNode }) {
  return (
    <div className="space-y-2">
      <div
        data-slot="panel-actions"
        className="flex flex-col gap-2 md:flex-row md:items-center md:gap-3 max-md:[&>*]:w-full"
      >
        {children}
      </div>
      {note && <p className="text-sm text-muted-foreground">{note}</p>}
    </div>
  );
}
