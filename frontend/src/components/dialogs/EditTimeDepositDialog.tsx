import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useUpdateTimeDeposit } from "@/hooks/useInvestments";
import { useSession } from "@/hooks/useSession";
import { PositionFormDialog } from "@/components/dialogs/PositionFormDialog";
import {
  TimeDepositFormBody,
  type TimeDepositFormState,
} from "@/components/dialogs/forms/TimeDepositForm";
import type { TimeDeposit, TimeDepositListItem } from "@/api/types";

type Props = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  timeDeposit: TimeDeposit | TimeDepositListItem;
};

function toForm(td: TimeDeposit | TimeDepositListItem): TimeDepositFormState {
  const i = td.investment;
  const d = td.details;
  return {
    display_name: i.display_name,
    description: i.description ?? "",
    ownership_type: i.ownership_type,
    entry_type: i.entry_type,
    sole_owner_user_id: i.sole_owner_user_id,
    risk_profile: td.investment.risk_profile,
    native_currency: i.native_currency,
    bank_name: d.bank_name,
    principal: d.principal,
    interest_rate: d.interest_rate,
    term_months: String(d.term_months),
    placement_date: d.placement_date ? d.placement_date.slice(0, 10) : "",
    maturity_date: d.maturity_date ? d.maturity_date.slice(0, 10) : "",
    rollover_policy: d.rollover_policy,
  };
}

export function EditTimeDepositDialog({ open, onOpenChange, timeDeposit }: Props) {
  const { t } = useTranslation(["investments", "common"]);
  const [form, setForm] = useState(() => toForm(timeDeposit));
  const { data: user } = useSession();
  const mutation = useUpdateTimeDeposit(timeDeposit.investment.id);

  const effectiveSoleOwnerID = form.sole_owner_user_id ?? user?.id ?? null;

  // Maturity must stay strictly after placement (issue #62) — mirrors the
  // server's ErrInvalidDepositTerm. (A term that strands existing snapshots is
  // caught server-side as OUTSIDE_DEPOSIT_TERM and surfaced via mutation.error.)
  const termInvalid =
    !!form.placement_date && !!form.maturity_date && form.maturity_date <= form.placement_date;

  function submit(close: () => void) {
    if (!form.risk_profile) return;
    if (termInvalid) return;
    mutation.mutate(
      {
        display_name: form.display_name,
        description: form.description || null,
        ownership_type: form.ownership_type,
        entry_type: form.entry_type,
        sole_owner_user_id: form.ownership_type === "sole" ? effectiveSoleOwnerID : null,
        risk_profile: form.risk_profile,
        bank_name: form.bank_name,
        principal: form.principal,
        interest_rate: form.interest_rate,
        term_months: Number(form.term_months),
        placement_date: form.placement_date,
        maturity_date: form.maturity_date,
        rollover_policy: form.rollover_policy,
      },
      { onSuccess: close },
    );
  }

  return (
    <PositionFormDialog
      open={open}
      onOpenChange={onOpenChange}
      formClassName="space-y-4"
      title={t("investments:timeDeposit.editTitle")}
      description={t("investments:timeDeposit.editDescription")}
      submitLabel={t("common:actions.saveChanges")}
      pendingLabel={t("common:actions.saving")}
      isPending={mutation.isPending}
      validate={() =>
        termInvalid ? { td_maturity_date: t("investments:timeDeposit.maturityAfterPlacement") } : {}
      }
      error={mutation.error}
      onSubmit={submit}
    >
      <TimeDepositFormBody
        mode="edit"
        form={form}
        setForm={setForm}
        soleOwnerID={effectiveSoleOwnerID}
      />
    </PositionFormDialog>
  );
}
