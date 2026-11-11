import { useTranslation } from "react-i18next";
import type { EntryFieldValues, EntryShape } from "@/components/entry/shapes";

// Split-renderer contract for one bulk-entry row (ADR-0050). EntryScreen owns
// all data / dirty-tracking / per-row field state / batch validation / atomic
// Save, and delegates only the row to EntryRowDesktop (cramped horizontal row)
// or EntryRowMobile (stacked fields). Both consume this one presentation-neutral
// projection plus the same onFieldChange/onReset callbacks — no logic forks into
// a renderer, and both expose the *same* data-testids so one spec asserts either.

// The carry-forward surfacing state for a row's "when" line.
export type EntryWhen =
  | { kind: "overwrite"; current: string } // a snapshot already exists for the chosen month (formatted value)
  | { kind: "carried"; month: string } // value carried from an earlier month (formatted)
  | { kind: "none" }; // no history yet

export type EntryRowView = {
  positionId: string;
  displayName: string;
  currency: string;
  // True when a value differs from the prefill: the only state that keeps the
  // dot and ring.
  dirty: boolean;
  // True when the user ticked "same as" on a carried row (#789): it saves its
  // prefill as this month's Snapshot without being a change.
  unchanged: boolean;
  ownership: string;
  when: EntryWhen;
  error: boolean;
  // Shown value per field key (the user's edit, else the carry-forward prefill).
  fieldValues: EntryFieldValues;
  // The derived-value column (qty×price computes value, accrued computes
  // principal): the currency symbol and the formatted number, split so the
  // renderer can align the symbol left and the number right. `value` is null
  // while the inputs are incomplete (renderer shows "—"); the whole field is
  // null when the shape has no derived column at all (amount-only).
  derived: { symbol: string; value: string | null } | null;
  // A soft warning on the row (a time deposit's Value below its principal, #844).
  // Never blocks Save.
  warning: boolean;
};

export type EntryRowRendererProps = {
  view: EntryRowView;
  shape: EntryShape;
  tid: string;
  // Copy prefix ("bulkEntry" or a group override) for the per-row error string.
  copy: string;
  onFieldChange: (fieldKey: string, value: string) => void;
  onReset: () => void;
  onUnchanged: (checked: boolean) => void;
};

// A native checkbox (the brand accent themes it, #780) that can show the
// indeterminate state a "tick all" header needs. The DOM property is set from a
// ref callback, not an effect.
export function SameCheckbox({
  checked,
  indeterminate = false,
  onChange,
  label,
  testId,
  className = "size-4",
}: {
  checked: boolean;
  indeterminate?: boolean;
  onChange: (checked: boolean) => void;
  label: string;
  testId: string;
  className?: string;
}) {
  return (
    <input
      type="checkbox"
      className={`${className} shrink-0 cursor-pointer accent-primary`}
      checked={checked}
      ref={(el) => {
        if (el) el.indeterminate = indeterminate;
      }}
      onChange={(e) => onChange(e.target.checked)}
      aria-label={label}
      data-testid={testId}
    />
  );
}

// The per-row "same as {month}" checkbox. Checking it is "mark Unchanged".
export function RowSameCheckbox({
  view,
  tid,
  className,
  onUnchanged,
}: {
  view: EntryRowView;
  tid: string;
  className?: string;
  onUnchanged: (checked: boolean) => void;
}) {
  const { t } = useTranslation("common");
  if (view.when.kind !== "carried" || view.dirty) return null;
  return (
    <SameCheckbox
      checked={view.unchanged}
      onChange={onUnchanged}
      label={t("bulkEntry.sameAriaLabel", { name: view.displayName, month: view.when.month })}
      testId={`${tid}-entry-same-${view.positionId}`}
      className={className}
    />
  );
}

// The name / ownership / "when" / per-row-error block. Identical in both
// renderers (same testids), so shared here; only its placement diverges — a
// left column on desktop, the top of the stack on mobile.
export function EntryRowMeta({
  view,
  tid,
  copy,
}: {
  view: EntryRowView;
  tid: string;
  copy: string;
}) {
  const { t } = useTranslation("common");
  return (
    <>
      <div className="flex items-center gap-1.5">
        {view.dirty && (
          <span
            className="size-1.5 shrink-0 rounded-full bg-primary"
            data-testid={`${tid}-entry-dirty-${view.positionId}`}
            aria-hidden
          />
        )}
        <span className="truncate font-medium">{view.displayName}</span>
      </div>
      <div className="text-xs text-muted-foreground">
        {view.ownership}
        {" · "}
        {view.when.kind === "overwrite" ? (
          <span data-testid={`${tid}-entry-current-${view.positionId}`}>
            {t("bulkEntry.current", { value: view.when.current })}
          </span>
        ) : view.when.kind === "carried" ? (
          view.unchanged ? (
            <span data-testid={`${tid}-entry-unchanged-${view.positionId}`}>
              {t("bulkEntry.sameAs", { month: view.when.month })}
            </span>
          ) : (
            t("bulkEntry.carriedFrom", { month: view.when.month })
          )
        ) : (
          t("bulkEntry.noHistory")
        )}
      </div>
      {view.warning && (
        <div
          className="text-xs text-warning"
          role="status"
          data-testid={`${tid}-entry-warning-${view.positionId}`}
        >
          {t("bulkEntry.belowPrincipal")}
        </div>
      )}
      {view.error && (
        <div
          className="text-xs text-destructive"
          data-testid={`${tid}-entry-error-${view.positionId}`}
        >
          {t(`${copy}.rowError`)}
        </div>
      )}
    </>
  );
}
