import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useCreateStock } from "@/hooks/useInvestments";
import { useSession } from "@/hooks/useSession";
import { RiskProfileSelect } from "@/components/common/RiskProfileSelect";
import { PositionFormDialog } from "@/components/dialogs/PositionFormDialog";
import { OwnershipField } from "@/components/common/OwnershipField";
import { EntryTypeField } from "@/components/common/EntryTypeField";
import type { RiskProfile, EntryType } from "@/api/types";

function emptyForm() {
  return {
    display_name: "",
    description: "",
    ownership_type: "joint" as "sole" | "joint",
    entry_type: "acquired" as EntryType,
    sole_owner_user_id: null as string | null,
    native_currency: "IDR",
    risk_profile: "" as RiskProfile | "",
    ticker: "",
    exchange: "",
  };
}

export function CreateStockDialog() {
  const { t } = useTranslation(["investments", "common"]);
  const [form, setForm] = useState(emptyForm);
  const { data: user } = useSession();
  const mutation = useCreateStock();

  const effectiveSoleOwnerID = form.sole_owner_user_id ?? user?.id ?? null;

  function submit(close: () => void) {
    if (!user) return;
    if (!form.risk_profile) return; // required, no default — see RiskProfileSelect
    mutation.mutate(
      {
        display_name: form.display_name,
        description: form.description || null,
        ownership_type: form.ownership_type,
        entry_type: form.entry_type,
        sole_owner_user_id: form.ownership_type === "sole" ? effectiveSoleOwnerID : null,
        native_currency: form.native_currency,
        risk_profile: form.risk_profile,
        ticker: form.ticker.toUpperCase(),
        exchange: form.exchange.toUpperCase(),
      },
      { onSuccess: close },
    );
  }

  return (
    <PositionFormDialog
      trigger={
        <Button>
          <Plus className="mr-1 size-4" />
          {t("investments:stock.createTrigger")}
        </Button>
      }
      title={t("investments:stock.createTitle")}
      description={t("investments:stock.createDescription")}
      submitLabel={t("common:actions.create")}
      pendingLabel={t("common:actions.creating")}
      isPending={mutation.isPending}
      error={mutation.error}
      onSubmit={submit}
      onClosed={() => {
        setForm(emptyForm());
        mutation.reset();
      }}
    >
      <div className="grid gap-2">
        <Label htmlFor="stock_display_name">{t("common:fields.displayName")}</Label>
        <Input
          id="stock_display_name"
          required
          value={form.display_name}
          onChange={(e) => setForm({ ...form, display_name: e.target.value })}
          placeholder={t("investments:stock.placeholders.displayName")}
        />
      </div>

      <div className="grid grid-cols-2 gap-3 [&>*]:content-end">
        <div className="grid gap-2">
          <Label htmlFor="stock_ticker">{t("investments:stock.fields.ticker")}</Label>
          <Input
            id="stock_ticker"
            required
            value={form.ticker}
            onChange={(e) => setForm({ ...form, ticker: e.target.value.toUpperCase() })}
            placeholder={t("investments:stock.placeholders.ticker")}
          />
        </div>
        <div className="grid gap-2">
          <Label htmlFor="stock_exchange">{t("investments:stock.fields.exchange")}</Label>
          <Input
            id="stock_exchange"
            required
            value={form.exchange}
            onChange={(e) => setForm({ ...form, exchange: e.target.value.toUpperCase() })}
            placeholder={t("investments:stock.placeholders.exchange")}
          />
        </div>
      </div>

      <div className="grid gap-2">
        <Label htmlFor="stock_currency">{t("common:fields.currency")}</Label>
        <Input
          id="stock_currency"
          required
          value={form.native_currency}
          onChange={(e) =>
            setForm({
              ...form,
              native_currency: e.target.value.toUpperCase(),
            })
          }
          placeholder={t("investments:stock.placeholders.currency")}
          maxLength={3}
        />
      </div>

      <OwnershipField
        idPrefix="stock_create"
        value={form.ownership_type}
        onChange={(v) => setForm({ ...form, ownership_type: v })}
        soleOwnerID={effectiveSoleOwnerID}
        onSoleOwnerChange={(v) => setForm({ ...form, sole_owner_user_id: v })}
      />

      <EntryTypeField
        idPrefix="stock_create"
        group="investment"
        value={form.entry_type}
        onChange={(v) => setForm({ ...form, entry_type: v })}
      />

      <RiskProfileSelect
        idPrefix="stock_create"
        value={form.risk_profile}
        onChange={(v) => setForm({ ...form, risk_profile: v })}
      />

      <div className="grid gap-2">
        <Label htmlFor="stock_description">{t("common:fields.description")}</Label>
        <Input
          id="stock_description"
          value={form.description}
          onChange={(e) => setForm({ ...form, description: e.target.value })}
        />
      </div>
    </PositionFormDialog>
  );
}
