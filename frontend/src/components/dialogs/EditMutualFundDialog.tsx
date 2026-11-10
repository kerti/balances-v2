import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useUpdateMutualFund } from "@/hooks/useInvestments";
import { useSession } from "@/hooks/useSession";
import { PositionFormDialog } from "@/components/dialogs/PositionFormDialog";
import {
  MutualFundFormBody,
  type MutualFundFormState,
} from "@/components/dialogs/forms/MutualFundForm";
import type { MutualFund, MutualFundListItem } from "@/api/types";

type Props = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  mutualFund: MutualFund | MutualFundListItem;
};

function toForm(m: MutualFund | MutualFundListItem): MutualFundFormState {
  return {
    display_name: m.investment.display_name,
    description: m.investment.description ?? "",
    ownership_type: m.investment.ownership_type,
    entry_type: m.investment.entry_type,
    sole_owner_user_id: m.investment.sole_owner_user_id,
    risk_profile: m.investment.risk_profile,
    native_currency: m.investment.native_currency,
    fund_code: m.details.fund_code,
    fund_manager: m.details.fund_manager ?? "",
    fund_type: m.details.fund_type,
  };
}

export function EditMutualFundDialog({ open, onOpenChange, mutualFund }: Props) {
  const { t } = useTranslation(["investments", "common"]);
  const mutation = useUpdateMutualFund(mutualFund.investment.id);
  const { data: user } = useSession();
  const [form, setForm] = useState(() => toForm(mutualFund));

  const effectiveSoleOwnerID = form.sole_owner_user_id ?? user?.id ?? null;

  function submit(close: () => void) {
    if (!form.risk_profile) return;
    if (!form.fund_type) return;
    mutation.mutate(
      {
        display_name: form.display_name,
        description: form.description || null,
        ownership_type: form.ownership_type,
        entry_type: form.entry_type,
        sole_owner_user_id: form.ownership_type === "sole" ? effectiveSoleOwnerID : null,
        risk_profile: form.risk_profile,
        fund_code: form.fund_code,
        fund_manager: form.fund_manager || null,
        fund_type: form.fund_type,
      },
      { onSuccess: close },
    );
  }

  return (
    <PositionFormDialog
      open={open}
      onOpenChange={onOpenChange}
      title={t("investments:mutualFund.editTitle")}
      description={t("investments:mutualFund.editDescription")}
      submitLabel={t("common:actions.saveChanges")}
      pendingLabel={t("common:actions.saving")}
      isPending={mutation.isPending}
      error={mutation.error}
      onSubmit={submit}
    >
      <MutualFundFormBody
        mode="edit"
        form={form}
        setForm={setForm}
        soleOwnerID={effectiveSoleOwnerID}
      />
    </PositionFormDialog>
  );
}
