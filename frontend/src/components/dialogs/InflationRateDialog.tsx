import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { MonthPickerPopover } from "@/components/common/MonthPickerPopover";
import { NumericInput } from "@/components/common/NumericInput";
import { PositionFormDialog } from "@/components/dialogs/PositionFormDialog";
import { useCreateInflationRate, useUpdateInflationRate } from "@/hooks/useInflationRates";
import { yearMonthsUpToNow } from "@/lib/dateLimits";
import {
  formatYearMonth,
  groupCanonical,
  INFLATION_DECIMALS,
  INFLATION_INTEGER_DIGITS,
} from "@/lib/format";
import type { InflationRate } from "@/api/types";
import type { FormMode } from "@/components/dialogs/forms/mode";

type Props =
  | { mode: "create" }
  | { mode: "edit"; rate: InflationRate; open: boolean; onOpenChange: (open: boolean) => void };

type InflationFormState = { year_month: string; rate: string };

const empty: InflationFormState = { year_month: "", rate: "" };

function InflationRateFormBody({
  mode,
  form,
  setForm,
}: {
  mode: FormMode;
  form: InflationFormState;
  setForm: (next: InflationFormState) => void;
}) {
  const { t } = useTranslation("settings");
  return (
    <>
      <div className="grid gap-2">
        <Label htmlFor="inflation-month">{t("inflation.month")}</Label>
        {mode === "create" ? (
          <MonthPickerPopover
            id="inflation-month"
            testId="inflation-month"
            months={yearMonthsUpToNow()}
            selected={form.year_month}
            onSelect={(ym) => setForm({ ...form, year_month: ym })}
            placeholder={t("inflation.pickMonth")}
          />
        ) : (
          <Input id="inflation-month" disabled value={formatYearMonth(form.year_month)} />
        )}
      </div>
      <div className="grid gap-2">
        <Label htmlFor="inflation-rate">{t("inflation.rate")}</Label>
        <NumericInput
          id="inflation-rate"
          // Example numeric rate, in the locale's marks; not translatable copy.
          placeholder={groupCanonical("3.5")}
          decimals={INFLATION_DECIMALS}
          maxIntegerDigits={INFLATION_INTEGER_DIGITS}
          allowNegative
          value={form.rate}
          onChange={(v) => setForm({ ...form, rate: v })}
        />
      </div>
    </>
  );
}

// One dialog for adding and editing a monthly inflation figure. Edit changes
// only the rate — the month is the row's identity — so it shows disabled in
// the same slot.
export function InflationRateDialog(props: Props) {
  const { t } = useTranslation(["settings", "common"]);
  const createRate = useCreateInflationRate();
  const updateRate = useUpdateInflationRate();
  const [form, setForm] = useState<InflationFormState>(() =>
    props.mode === "edit" ? { year_month: props.rate.year_month, rate: props.rate.rate } : empty,
  );

  // A rate may be negative (deflation) or zero; only require a parseable number.
  const valid =
    form.rate.trim() !== "" &&
    !Number.isNaN(Number(form.rate)) &&
    (props.mode === "edit" || form.year_month !== "");

  function submit(close: () => void) {
    if (props.mode === "edit") {
      updateRate.mutate({ id: props.rate.id, rate: form.rate }, { onSuccess: close });
      return;
    }
    createRate.mutate({ year_month: form.year_month, rate: form.rate }, { onSuccess: close });
  }

  const body = <InflationRateFormBody mode={props.mode} form={form} setForm={setForm} />;

  if (props.mode === "edit") {
    return (
      <PositionFormDialog
        open={props.open}
        onOpenChange={props.onOpenChange}
        title={t("settings:inflation.editTitle")}
        description={t("settings:inflation.editDescription")}
        submitLabel={t("common:actions.saveChanges")}
        pendingLabel={t("common:actions.saving")}
        isPending={updateRate.isPending}
        submitDisabled={!valid}
        error={updateRate.error}
        onSubmit={submit}
      >
        {body}
      </PositionFormDialog>
    );
  }

  return (
    <PositionFormDialog
      trigger={
        <Button size="sm" data-testid="inflation-new-rate">
          <Plus className="mr-1 size-4" />
          {t("settings:inflation.newRate")}
        </Button>
      }
      title={t("settings:inflation.createTitle")}
      description={t("settings:inflation.createDescription")}
      submitLabel={t("settings:inflation.addRate")}
      pendingLabel={t("common:actions.creating")}
      isPending={createRate.isPending}
      submitDisabled={!valid}
      error={createRate.error}
      onSubmit={submit}
      onClosed={() => {
        setForm(empty);
        createRate.reset();
      }}
    >
      {body}
    </PositionFormDialog>
  );
}
