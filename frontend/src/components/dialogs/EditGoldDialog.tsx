import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useUpdateGold, type GoldForm } from "@/hooks/useInvestments";
import { useSession } from "@/hooks/useSession";
import { PositionFormDialog } from "@/components/dialogs/PositionFormDialog";
import { GoldFormBody, type GoldFormState } from "@/components/dialogs/forms/GoldForm";
import type { Gold, GoldListItem } from "@/api/types";

type Props = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  gold: Gold | GoldListItem;
};

function toForm(g: Gold | GoldListItem): GoldFormState {
  return {
    display_name: g.investment.display_name,
    description: g.investment.description ?? "",
    ownership_type: g.investment.ownership_type,
    entry_type: g.investment.entry_type,
    sole_owner_user_id: g.investment.sole_owner_user_id,
    risk_profile: g.investment.risk_profile,
    native_currency: g.investment.native_currency,
    form: g.details.form as GoldForm,
    purity: g.details.purity,
  };
}

export function EditGoldDialog({ open, onOpenChange, gold }: Props) {
  const { t } = useTranslation(["investments", "common"]);
  const mutation = useUpdateGold(gold.investment.id);
  const { data: user } = useSession();
  const [form, setForm] = useState(() => toForm(gold));

  const effectiveSoleOwnerID = form.sole_owner_user_id ?? user?.id ?? null;

  function submit(close: () => void) {
    if (!form.risk_profile) return;
    mutation.mutate(
      {
        display_name: form.display_name,
        description: form.description || null,
        ownership_type: form.ownership_type,
        entry_type: form.entry_type,
        sole_owner_user_id: form.ownership_type === "sole" ? effectiveSoleOwnerID : null,
        risk_profile: form.risk_profile,
        form: form.form,
        purity: form.purity,
      },
      { onSuccess: close },
    );
  }

  return (
    <PositionFormDialog
      open={open}
      onOpenChange={onOpenChange}
      title={t("investments:gold.editTitle")}
      description={t("investments:gold.editDescription")}
      submitLabel={t("common:actions.saveChanges")}
      pendingLabel={t("common:actions.saving")}
      isPending={mutation.isPending}
      error={mutation.error}
      onSubmit={submit}
    >
      <GoldFormBody mode="edit" form={form} setForm={setForm} soleOwnerID={effectiveSoleOwnerID} />
    </PositionFormDialog>
  );
}
