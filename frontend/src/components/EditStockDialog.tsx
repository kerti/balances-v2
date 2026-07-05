import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useUpdateStock } from "@/hooks/useInvestments";
import { useHouseholdMembers } from "@/hooks/useHouseholdMembers";
import { preferredName } from "@/lib/names";
import { useSession } from "@/hooks/useSession";
import { RiskProfileSelect } from "@/components/RiskProfileSelect";
import { PositionFormDialog } from "@/components/PositionFormDialog";
import type { Stock, StockListItem } from "@/api/types";

type Props = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  // Accepts either the detail-page Stock or a list-row StockListItem so
  // both call sites can pass what they already have.
  stock: Stock | StockListItem;
};

function toForm(s: Stock | StockListItem) {
  return {
    display_name: s.investment.display_name,
    description: s.investment.description ?? "",
    ownership_type: s.investment.ownership_type,
    sole_owner_user_id: s.investment.sole_owner_user_id,
    risk_profile: s.investment.risk_profile,
    ticker: s.details.ticker,
    exchange: s.details.exchange,
  };
}

export function EditStockDialog({ open, onOpenChange, stock }: Props) {
  const { t } = useTranslation(["investments", "common"]);
  const mutation = useUpdateStock(stock.investment.id);
  const { data: user } = useSession();
  const { data: members } = useHouseholdMembers();
  const [form, setForm] = useState(() => toForm(stock));

  const effectiveSoleOwnerID = form.sole_owner_user_id ?? user?.id ?? null;

  function submit(close: () => void) {
    mutation.mutate(
      {
        display_name: form.display_name,
        description: form.description || null,
        ownership_type: form.ownership_type,
        sole_owner_user_id:
          form.ownership_type === "sole" ? effectiveSoleOwnerID : null,
        risk_profile: form.risk_profile,
        ticker: form.ticker.toUpperCase(),
        exchange: form.exchange.toUpperCase(),
      },
      { onSuccess: close },
    );
  }

  return (
    <PositionFormDialog
      open={open}
      onOpenChange={onOpenChange}
      contentClassName="max-h-[90vh] overflow-y-auto"
      title={t("investments:stock.editTitle")}
      description={t("investments:stock.editDescription")}
      submitLabel={t("common:actions.saveChanges")}
      pendingLabel={t("common:actions.saving")}
      isPending={mutation.isPending}
      error={mutation.error}
      onSubmit={submit}
    >
      <div className="grid gap-2">
        <Label htmlFor="edit_stock_display_name">
          {t("common:fields.displayName")}
        </Label>
        <Input
          id="edit_stock_display_name"
          required
          value={form.display_name}
          onChange={(e) => setForm({ ...form, display_name: e.target.value })}
        />
      </div>

      <div className="grid grid-cols-2 gap-3">
        <div className="grid gap-2">
          <Label htmlFor="edit_stock_ticker">
            {t("investments:stock.fields.ticker")}
          </Label>
          <Input
            id="edit_stock_ticker"
            required
            value={form.ticker}
            onChange={(e) =>
              setForm({ ...form, ticker: e.target.value.toUpperCase() })
            }
          />
        </div>
        <div className="grid gap-2">
          <Label htmlFor="edit_stock_exchange">
            {t("investments:stock.fields.exchange")}
          </Label>
          <Input
            id="edit_stock_exchange"
            required
            value={form.exchange}
            onChange={(e) =>
              setForm({ ...form, exchange: e.target.value.toUpperCase() })
            }
          />
        </div>
      </div>

      <div className="grid gap-2">
        <Label>{t("common:fields.ownership")}</Label>
        <div className="flex gap-4 text-sm">
          <label className="flex items-center gap-2">
            <input
              type="radio"
              name="edit_stock_ownership_type"
              value="joint"
              checked={form.ownership_type === "joint"}
              onChange={() => setForm({ ...form, ownership_type: "joint" })}
            />
            {t("investments:ownership.joint")}
          </label>
          <label className="flex items-center gap-2">
            <input
              type="radio"
              name="edit_stock_ownership_type"
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
            onChange={(e) =>
              setForm({ ...form, sole_owner_user_id: e.target.value })
            }
          >
            {(members ?? []).map((m) => (
              <option key={m.id} value={m.id}>
                {preferredName(m)}
                {user && m.id === user.id
                  ? t("common:ownership.youSuffix")
                  : ""}
              </option>
            ))}
          </select>
        )}
      </div>

      <div className="grid gap-2">
        <Label htmlFor="edit_stock_description">
          {t("common:fields.description")}
        </Label>
        <Input
          id="edit_stock_description"
          value={form.description}
          onChange={(e) => setForm({ ...form, description: e.target.value })}
        />
      </div>

      <RiskProfileSelect
        idPrefix="stock_edit"
        value={form.risk_profile}
        onChange={(v) => setForm({ ...form, risk_profile: v })}
      />
    </PositionFormDialog>
  );
}
