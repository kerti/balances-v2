import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useUpdateBond } from "@/hooks/useInvestments";
import { useSession } from "@/hooks/useSession";
import { PositionFormDialog } from "@/components/dialogs/PositionFormDialog";
import { BondFormBody, type BondFormState } from "@/components/dialogs/forms/BondForm";
import type { Bond, BondListItem } from "@/api/types";

type Props = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  bond: Bond | BondListItem;
};

function toForm(bond: Bond | BondListItem): BondFormState {
  const i = bond.investment;
  const d = bond.details;
  return {
    display_name: i.display_name,
    description: i.description ?? "",
    ownership_type: i.ownership_type,
    entry_type: i.entry_type,
    sole_owner_user_id: i.sole_owner_user_id,
    risk_profile: bond.investment.risk_profile,
    native_currency: i.native_currency,
    bond_type: d.bond_type,
    series_code: d.series_code ?? "",
    issuer: d.issuer,
    face_value: "",
    placement_date: "",
    coupon_rate: d.coupon_rate,
    coupon_frequency: d.coupon_frequency,
    coupon_disposition: d.coupon_disposition,
    maturity_date: d.maturity_date ? d.maturity_date.slice(0, 10) : "",
  };
}

export function EditBondDialog({ open, onOpenChange, bond }: Props) {
  const { t } = useTranslation(["investments", "common"]);
  const [form, setForm] = useState(() => toForm(bond));
  const { data: user } = useSession();
  const mutation = useUpdateBond(bond.investment.id);

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
        bond_type: form.bond_type,
        series_code: form.series_code.trim() || null,
        issuer: form.issuer,
        coupon_rate: form.coupon_rate,
        coupon_frequency: form.coupon_frequency,
        coupon_disposition: form.coupon_disposition,
        maturity_date: form.maturity_date,
      },
      { onSuccess: close },
    );
  }

  return (
    <PositionFormDialog
      open={open}
      onOpenChange={onOpenChange}
      formClassName="space-y-4"
      title={t("investments:bond.editTitle")}
      description={t("investments:bond.editDescription")}
      submitLabel={t("common:actions.saveChanges")}
      pendingLabel={t("common:actions.saving")}
      isPending={mutation.isPending}
      error={mutation.error}
      onSubmit={submit}
    >
      <BondFormBody mode="edit" form={form} setForm={setForm} soleOwnerID={effectiveSoleOwnerID} />
    </PositionFormDialog>
  );
}
