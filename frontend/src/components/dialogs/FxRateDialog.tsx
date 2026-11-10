import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { MonthPickerPopover } from "@/components/common/MonthPickerPopover";
import { NumericInput } from "@/components/common/NumericInput";
import { PositionFormDialog } from "@/components/dialogs/PositionFormDialog";
import { useCreateFxRate, useUpdateFxRate } from "@/hooks/useFxRates";
import { useSession } from "@/hooks/useSession";
import { yearMonthsUpToNow } from "@/lib/dateLimits";
import { fxEquation } from "@/lib/fxPair";
import { formatYearMonth, groupCanonical, RATE_DECIMALS, RATE_INTEGER_DIGITS } from "@/lib/format";
import type { FxRate } from "@/api/types";
import type { FormMode } from "@/components/dialogs/forms/mode";

type Props =
  | { mode: "create" }
  | { mode: "edit"; rate: FxRate; open: boolean; onOpenChange: (open: boolean) => void };

type FxRateFormState = { year_month: string; currency: string; rate: string };

const empty: FxRateFormState = { year_month: "", currency: "", rate: "" };

function FxRateFormBody({
  mode,
  form,
  setForm,
  base,
}: {
  mode: FormMode;
  form: FxRateFormState;
  setForm: (next: FxRateFormState) => void;
  base: string | undefined;
}) {
  const { t } = useTranslation("settings");
  return (
    <>
      <div className="grid gap-2">
        <Label htmlFor="fx-month">{t("fx.month")}</Label>
        {mode === "create" ? (
          <MonthPickerPopover
            id="fx-month"
            testId="fx-month"
            months={yearMonthsUpToNow()}
            selected={form.year_month}
            onSelect={(ym) => setForm({ ...form, year_month: ym })}
            placeholder={t("fx.pickMonth")}
          />
        ) : (
          <Input id="fx-month" disabled value={formatYearMonth(form.year_month)} />
        )}
      </div>
      <div className="grid gap-2">
        <Label htmlFor="fx-currency">{t("fx.currency")}</Label>
        <Input
          id="fx-currency"
          className="uppercase"
          maxLength={3}
          disabled={mode === "edit"}
          // ISO currency code — a data token, not translatable copy.
          placeholder={"USD"}
          value={form.currency}
          onChange={(e) => setForm({ ...form, currency: e.target.value })}
        />
      </div>
      <div className="grid gap-2">
        <Label htmlFor="fx-rate">{t("fx.rate")}</Label>
        <NumericInput
          id="fx-rate"
          // Example numeric rate, in the locale's marks; not translatable copy.
          placeholder={groupCanonical("16000")}
          decimals={RATE_DECIMALS}
          maxIntegerDigits={RATE_INTEGER_DIGITS}
          value={form.rate}
          onChange={(v) => setForm({ ...form, rate: v })}
        />
      </div>
      {/* Live direction hint: once a currency code is entered, spell out which
          way the rate converts and into which currency. */}
      {base && form.currency.length === 3 && (
        <p className="text-sm text-muted-foreground" data-testid="fx-rate-hint">
          {fxEquation(form.currency.toUpperCase(), form.rate.trim() || "?", base)}
        </p>
      )}
    </>
  );
}

// One dialog for adding and editing a manual monthly FX rate (ADR-0024). Edit
// changes only the rate — month + currency are the row's identity — so those
// two show disabled in the same slots.
export function FxRateDialog(props: Props) {
  const { t } = useTranslation(["settings", "common"]);
  const { data: me } = useSession();
  const createRate = useCreateFxRate();
  const updateRate = useUpdateFxRate();
  const base = me?.reporting_currency?.toUpperCase();
  const [form, setForm] = useState<FxRateFormState>(() =>
    props.mode === "edit"
      ? { year_month: props.rate.year_month, currency: props.rate.currency, rate: props.rate.rate }
      : empty,
  );

  const valid =
    form.rate !== "" &&
    Number(form.rate) > 0 &&
    (props.mode === "edit" || (form.year_month !== "" && form.currency.length === 3));

  function submit(close: () => void) {
    if (props.mode === "edit") {
      updateRate.mutate({ id: props.rate.id, rate: form.rate }, { onSuccess: close });
      return;
    }
    createRate.mutate(
      { year_month: form.year_month, currency: form.currency.toUpperCase(), rate: form.rate },
      { onSuccess: close },
    );
  }

  const body = <FxRateFormBody mode={props.mode} form={form} setForm={setForm} base={base} />;

  if (props.mode === "edit") {
    return (
      <PositionFormDialog
        open={props.open}
        onOpenChange={props.onOpenChange}
        title={t("settings:fx.editTitle")}
        description={t("settings:fx.editDescription")}
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
        <Button size="sm" data-testid="fx-new-rate">
          <Plus className="mr-1 size-4" />
          {t("settings:fx.newRate")}
        </Button>
      }
      title={t("settings:fx.createTitle")}
      description={t("settings:fx.createDescription")}
      submitLabel={t("settings:fx.addRate")}
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
