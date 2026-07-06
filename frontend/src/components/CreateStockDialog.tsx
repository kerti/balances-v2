import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useCreateStock } from "@/hooks/useInvestments";
import { useSession } from "@/hooks/useSession";
import { useHouseholdMembers } from "@/hooks/useHouseholdMembers";
import { preferredName } from "@/lib/names";
import { RiskProfileSelect } from "@/components/RiskProfileSelect";
import { PositionFormDialog } from "@/components/PositionFormDialog";
import type { RiskProfile } from "@/api/types";

function emptyForm() {
  return {
    display_name: "",
    description: "",
    ownership_type: "joint" as "sole" | "joint",
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
  const { data: members } = useHouseholdMembers();
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
      contentClassName="max-h-[90vh] overflow-y-auto"
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

      <div className="grid grid-cols-2 gap-3">
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

      <div className="grid gap-2">
        <Label>{t("common:fields.ownership")}</Label>
        <div className="flex gap-4 text-sm">
          <label className="flex items-center gap-2">
            <input
              type="radio"
              name="ownership_type"
              value="joint"
              checked={form.ownership_type === "joint"}
              onChange={() => setForm({ ...form, ownership_type: "joint" })}
            />
            {t("investments:ownership.joint")}
          </label>
          <label className="flex items-center gap-2">
            <input
              type="radio"
              name="ownership_type"
              value="sole"
              checked={form.ownership_type === "sole"}
              onChange={() => setForm({ ...form, ownership_type: "sole" })}
            />
            {t("investments:ownership.soleOwner")}
          </label>
        </div>
        {form.ownership_type === "sole" && (
          <select
            aria-label={t("investments:ownership.soleOwnerAria")}
            className="h-9 rounded-md border border-input bg-background px-3 text-sm"
            value={effectiveSoleOwnerID ?? ""}
            onChange={(e) => setForm({ ...form, sole_owner_user_id: e.target.value })}
          >
            {(members ?? []).map((m) => (
              <option key={m.id} value={m.id}>
                {preferredName(m)}
                {user && m.id === user.id ? t("common:ownership.youSuffix") : ""}
              </option>
            ))}
          </select>
        )}
      </div>

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
