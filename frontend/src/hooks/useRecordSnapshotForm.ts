import { useState } from "react";
import {
  defaultRecordMonth,
  lastRecordedBefore,
  snapshotForMonth,
  ymOf,
} from "@/lib/recordSnapshot";

type Dated = { year_month: string };
type Dateable = { year_month: string; as_of_date: string };

// Form state for the Record snapshot dialogs (#788). The dialog opens on the
// next missing month; picking a month that already has a snapshot switches it
// to replace mode, prefilled with that snapshot's values; picking an empty
// month leaves the value fields empty. `existing` and `last` are derived from
// the live snapshot list on every render, so a refetch while the dialog is open
// is picked up without an effect.
export function useRecordSnapshotForm<S extends Dated, F extends Dateable>({
  snapshots,
  emptyForm,
  formFrom,
}: {
  snapshots: S[] | undefined;
  emptyForm: (yearMonth: string) => F;
  formFrom: (snapshot: S, yearMonth: string) => F;
}) {
  const [form, setForm] = useState<F>(() => emptyForm(defaultRecordMonth(snapshots)));

  const existing = snapshotForMonth(snapshots, form.year_month);
  // "Same as last month" only makes sense for an empty month.
  const last = existing ? undefined : lastRecordedBefore(snapshots, form.year_month);

  function startFor(yearMonth: string): F {
    const found = snapshotForMonth(snapshots, yearMonth);
    return found ? formFrom(found, yearMonth) : emptyForm(yearMonth);
  }

  // A fresh form on the default month, for each time the dialog opens. `prefill`
  // overrides individual fields (the "Update holding" shortcut, #830); it is a
  // starting point the user confirms or edits, never a write.
  function reset(prefill?: Partial<F>) {
    setForm({ ...startFor(defaultRecordMonth(snapshots)), ...prefill });
  }

  function pickMonth(yearMonth: string) {
    if (snapshotForMonth(snapshots, yearMonth)) {
      setForm(startFor(yearMonth));
    } else if (existing) {
      // Leaving a replace for an empty month: the prefilled values belonged to
      // the month being left.
      setForm(emptyForm(yearMonth));
    } else {
      // Empty to empty keeps what was typed; a statement date outside the new
      // month would trip the month-integrity CHECK, so it is cleared.
      setForm({
        ...form,
        year_month: yearMonth,
        as_of_date: form.as_of_date.startsWith(ymOf(yearMonth)) ? form.as_of_date : "",
      });
    }
  }

  return { form, setForm, existing, last, pickMonth, reset };
}
