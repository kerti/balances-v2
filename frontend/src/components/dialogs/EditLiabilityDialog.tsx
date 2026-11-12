import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useUpdateLiability } from "@/hooks/useLiabilities";
import { useSession } from "@/hooks/useSession";
import { PositionFormDialog } from "@/components/dialogs/PositionFormDialog";
import {
  LiabilityFormBody,
  type LiabilityFormState,
} from "@/components/dialogs/forms/LiabilityForm";
import type { Liability } from "@/api/types";

type Props = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  liability: Liability;
};

function toForm(l: Liability): LiabilityFormState {
  return {
    display_name: l.display_name,
    description: l.description ?? "",
    subtype: l.subtype,
    ownership_type: l.ownership_type,
    entry_type: l.entry_type,
    sole_owner_user_id: l.sole_owner_user_id,
    native_currency: l.native_currency,
    counterparty_name: l.counterparty_name,
    principal: l.principal ?? "",
    interest_rate: l.interest_rate ?? "",
    term_months: l.term_months !== null ? String(l.term_months) : "",
    start_date: l.start_date ? l.start_date.slice(0, 10) : "",
    maturity_date: l.maturity_date ? l.maturity_date.slice(0, 10) : "",
  };
}

export function EditLiabilityDialog({ open, onOpenChange, liability }: Props) {
  const { t } = useTranslation(["liabilities", "common"]);
  const mutation = useUpdateLiability(liability.id);
  const { data: user } = useSession();
  const [form, setForm] = useState(() => toForm(liability));
  const [tagId, setTagId] = useState<string | null>(() => liability.tag_id);

  const effectiveSoleOwnerID = form.sole_owner_user_id ?? user?.id ?? null;

  function submit(close: () => void) {
    mutation.mutate(
      {
        display_name: form.display_name,
        description: form.description || null,
        ownership_type: form.ownership_type,
        entry_type: form.entry_type,
        tag_id: tagId,
        sole_owner_user_id: form.ownership_type === "sole" ? effectiveSoleOwnerID : null,
        counterparty_name: form.counterparty_name,
        principal: form.principal || null,
        interest_rate: form.interest_rate || null,
        term_months: form.term_months ? Number(form.term_months) : null,
        start_date: form.start_date || null,
        maturity_date: form.maturity_date || null,
      },
      { onSuccess: close },
    );
  }

  return (
    <PositionFormDialog
      open={open}
      onOpenChange={onOpenChange}
      title={t("liabilities:editTitle")}
      description={t("liabilities:editDescription")}
      submitLabel={t("common:actions.saveChanges")}
      pendingLabel={t("common:actions.saving")}
      isPending={mutation.isPending}
      tag={{ value: tagId, onChange: setTagId }}
      error={mutation.error}
      onSubmit={submit}
    >
      <LiabilityFormBody
        mode="edit"
        form={form}
        setForm={setForm}
        soleOwnerID={effectiveSoleOwnerID}
      />
    </PositionFormDialog>
  );
}
