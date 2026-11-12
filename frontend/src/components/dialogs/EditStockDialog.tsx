import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useUpdateStock } from "@/hooks/useInvestments";
import { useSession } from "@/hooks/useSession";
import { PositionFormDialog } from "@/components/dialogs/PositionFormDialog";
import { StockFormBody, type StockFormState } from "@/components/dialogs/forms/StockForm";
import type { Stock, StockListItem } from "@/api/types";

type Props = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  // Accepts either the detail-page Stock or a list-row StockListItem so
  // both call sites can pass what they already have.
  stock: Stock | StockListItem;
};

function toForm(s: Stock | StockListItem): StockFormState {
  return {
    display_name: s.investment.display_name,
    description: s.investment.description ?? "",
    ownership_type: s.investment.ownership_type,
    entry_type: s.investment.entry_type,
    sole_owner_user_id: s.investment.sole_owner_user_id,
    native_currency: s.investment.native_currency,
    risk_profile: s.investment.risk_profile,
    ticker: s.details.ticker,
    exchange: s.details.exchange,
  };
}

export function EditStockDialog({ open, onOpenChange, stock }: Props) {
  const { t } = useTranslation(["investments", "common"]);
  const mutation = useUpdateStock(stock.investment.id);
  const { data: user } = useSession();
  const [form, setForm] = useState(() => toForm(stock));
  const [tagId, setTagId] = useState<string | null>(() => stock.investment.tag_id);

  const effectiveSoleOwnerID = form.sole_owner_user_id ?? user?.id ?? null;

  function submit(close: () => void) {
    if (!form.risk_profile) return;
    mutation.mutate(
      {
        display_name: form.display_name,
        description: form.description || null,
        ownership_type: form.ownership_type,
        entry_type: form.entry_type,
        tag_id: tagId,
        sole_owner_user_id: form.ownership_type === "sole" ? effectiveSoleOwnerID : null,
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
      title={t("investments:stock.editTitle")}
      description={t("investments:stock.editDescription")}
      submitLabel={t("common:actions.saveChanges")}
      pendingLabel={t("common:actions.saving")}
      isPending={mutation.isPending}
      tag={{ value: tagId, onChange: setTagId }}
      error={mutation.error}
      onSubmit={submit}
    >
      <StockFormBody mode="edit" form={form} setForm={setForm} soleOwnerID={effectiveSoleOwnerID} />
    </PositionFormDialog>
  );
}
