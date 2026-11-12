import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useCreateBond } from "@/hooks/useInvestments";
import { useSession } from "@/hooks/useSession";
import { PositionFormDialog } from "@/components/dialogs/PositionFormDialog";
import { BondFormBody } from "@/components/dialogs/forms/BondForm";
import type { RiskProfile, EntryType } from "@/api/types";
import type { BondType, CouponFrequency, CouponDisposition } from "@/api/types";

function emptyForm() {
  return {
    display_name: "",
    description: "",
    ownership_type: "sole" as "sole" | "joint",
    entry_type: "acquired" as EntryType,
    sole_owner_user_id: null as string | null,
    risk_profile: "" as RiskProfile | "",
    native_currency: "IDR",
    bond_type: "govt_primary" as BondType,
    series_code: "",
    issuer: "",
    face_value: "",
    placement_date: "",
    coupon_rate: "",
    coupon_frequency: "monthly" as CouponFrequency,
    coupon_disposition: "pays_out" as CouponDisposition,
    maturity_date: "",
  };
}

type Props = {
  // Called with the new position's id after a successful create, so the list
  // can land on its detail page (#843).
  onCreated?: (id: string) => void;
};

export function CreateBondDialog({ onCreated }: Props) {
  const { t } = useTranslation(["investments", "common"]);
  const [form, setForm] = useState(emptyForm);
  const { data: user } = useSession();
  const mutation = useCreateBond();
  const [tagId, setTagId] = useState<string | null>(null);

  const effectiveSoleOwnerID = form.sole_owner_user_id ?? user?.id ?? null;

  function submit(close: () => void) {
    if (!user) return;
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
        native_currency: form.native_currency,
        bond_type: form.bond_type,
        series_code: form.series_code.trim() || null,
        issuer: form.issuer,
        face_value: form.face_value,
        placement_date: form.placement_date,
        coupon_rate: form.coupon_rate,
        coupon_frequency: form.coupon_frequency,
        coupon_disposition: form.coupon_disposition,
        maturity_date: form.maturity_date,
      },
      {
        onSuccess: (created) => {
          close();
          onCreated?.(created.investment.id);
        },
      },
    );
  }

  return (
    <PositionFormDialog
      trigger={
        <Button>
          <Plus className="mr-1 size-4" />
          {t("investments:bond.createTrigger")}
        </Button>
      }
      formClassName="space-y-4"
      title={t("investments:bond.createTitle")}
      description={t("investments:bond.createDescription")}
      submitLabel={t("common:actions.create")}
      pendingLabel={t("common:actions.creating")}
      isPending={mutation.isPending}
      tag={{ value: tagId, onChange: setTagId }}
      error={mutation.error}
      onSubmit={submit}
      onClosed={() => {
        setForm(emptyForm());
        setTagId(null);
        mutation.reset();
      }}
    >
      <BondFormBody
        mode="create"
        form={form}
        setForm={setForm}
        soleOwnerID={effectiveSoleOwnerID}
      />
    </PositionFormDialog>
  );
}
