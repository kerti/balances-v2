import { useTranslation } from "react-i18next";
import { Label } from "@/components/ui/label";
import { FieldError, describedBy, useFieldError } from "@/components/ui/field-error";
import { entryTypeFieldId } from "@/components/common/entryTypeValidation";
import type { EntryType } from "@/api/types";

// The "where did this come from?" control, shared by all 20 position
// Create/Edit dialogs (#594, ADR-0053 §3). Built as one component from the start
// on the `OwnershipField` precedent — that control's twenty hand-inlined copies
// are why the mobile tap floor could not be fixed in one place (#541), and this
// is the same radio-pair shape.
//
// Two radios rather than a checkbox, deliberately: an unchecked box says
// nothing, and this needs an affirmative answer. Create forms pre-select
// nothing and block submit until one is picked, because each default silently
// misstates the first month's living-expenses estimate in one common case
// (#934). The DB default (`acquired`) remains for import and restore, which
// have no one to ask.
//
// It appears on Edit as well as Create, and that is load-bearing rather than a
// convenience: nothing detects a mis-declared entry — a one-sided birth is
// indistinguishable from an acquisition whose funding has not been snapshotted
// yet — so flipping this control is the only remedy, including for a declaration
// inherited from a restore or an import.

// The copy comes in two sets, because the *engine* question — "is the other leg
// of this birth already in the books?" — has two different real-world answers
// depending on whether the household owns the position or owes it.
//
// For something owned, the other leg is the money that paid for it: you bought
// the fund out of a tracked account. For something owed, nothing funded it —
// a debt is taken on, and what makes its birth two-sided is that the money it
// released, or the thing it bought, landed in the books as well. ADR-0053 §3
// quotes only the owned wording; asking a household where a mortgage was
// "funded from" reads as nonsense, which is what this split fixes.
type CopySet = "owned" | "owed";

type PositionGroup = "asset" | "liability" | "receivable" | "investment";

type Props = {
  /** Disambiguating prefix for the radio group name, e.g. "td_create". */
  idPrefix: string;
  /**
   * The position's group. Callers pass their own group rather than picking a
   * wording, so the owned/owed mapping stays in one place and a future group
   * that needs its own phrasing is a change here, not at twenty callsites.
   */
  group: PositionGroup;
  /** `null` on Create until the user answers: neither radio is checked. */
  value: EntryType | null;
  onChange: (next: EntryType) => void;
  /** Overrides the message the surrounding `ValidatedForm` supplies for this field. */
  error?: string;
};

export function EntryTypeField({ idPrefix, group, value, onChange, error }: Props) {
  const { t } = useTranslation("common");
  const name = entryTypeFieldId(idPrefix);
  const fieldError = useFieldError(name);
  const message = error ?? fieldError.message;
  const copy: CopySet = group === "liability" ? "owed" : "owned";

  return (
    <div className="grid gap-2">
      <Label required>{t(`entry.${copy}.question`)}</Label>
      <div data-testid="entry-type-options" className="grid gap-2 text-sm">
        {(["acquired", "newlyTracked"] as const).map((option, index) => {
          const optionValue: EntryType = option === "acquired" ? "acquired" : "newly_tracked";
          return (
            <label
              key={option}
              className="flex items-center gap-2 max-md:min-h-11 max-md:rounded-lg max-md:border max-md:border-input max-md:px-3"
            >
              <input
                type="radio"
                id={index === 0 ? name : undefined}
                aria-invalid={message !== undefined || undefined}
                aria-describedby={describedBy(undefined, name, message !== undefined)}
                name={name}
                value={optionValue}
                checked={value === optionValue}
                onChange={() => onChange(optionValue)}
              />
              {t(`entry.${copy}.${option}`)}
            </label>
          );
        })}
      </div>
      {message && <FieldError id={name}>{message}</FieldError>}
      <p className="text-xs text-muted-foreground">{t(`entry.${copy}.hint`)}</p>
    </div>
  );
}
