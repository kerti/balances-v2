import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useCreateTimeDeposit } from "@/hooks/useInvestments";
import { useSession } from "@/hooks/useSession";
import { PositionFormDialog } from "@/components/dialogs/PositionFormDialog";
import { TimeDepositFormBody } from "@/components/dialogs/forms/TimeDepositForm";
import type { TimeDepositForm } from "@/lib/rollover";

function emptyForm(): TimeDepositForm {
  return {
    display_name: "",
    description: "",
    ownership_type: "sole",
    entry_type: "acquired",
    sole_owner_user_id: null,
    risk_profile: "",
    native_currency: "IDR",
    bank_name: "",
    principal: "",
    interest_rate: "",
    term_months: "",
    placement_date: "",
    maturity_date: "",
    rollover_policy: "no_rollover",
  };
}

// Bank-actual maturities sometimes nudge by a day or two (holidays); the
// computed maturity_date is overwritten on any change to placement / term, but
// the user can edit it directly afterward and the override sticks until they
// touch placement or term again. The date math lives in lib/rollover.

type Props = {
  // Seed the form (e.g. the matured-TD rollover helper). Reset target on close.
  prefill?: Partial<TimeDepositForm>;
  // When this deposit redeploys a matured position's funds, the source TD's id
  // (issue #29). Sent on create so the source's rollover callout disappears.
  rolledFromInvestmentId?: string;
  triggerLabel?: string;
  triggerVariant?: "default" | "outline";
  triggerSize?: "default" | "sm";
  // Called with the new deposit's id after a successful create (#843).
  onCreated?: (id: string) => void;
};

export function CreateTimeDepositDialog({
  prefill,
  rolledFromInvestmentId,
  triggerLabel,
  triggerVariant = "default",
  triggerSize = "default",
  onCreated,
}: Props = {}) {
  const { t } = useTranslation(["investments", "common"]);
  const [form, setForm] = useState<TimeDepositForm>(() => ({
    ...emptyForm(),
    ...prefill,
  }));
  const { data: user } = useSession();
  const mutation = useCreateTimeDeposit();
  const [tagId, setTagId] = useState<string | null>(null);

  const effectiveSoleOwnerID = form.sole_owner_user_id ?? user?.id ?? null;

  // The term must be a forward window (issue #62): maturity strictly after
  // placement. Mirrors the server's ErrInvalidDepositTerm so the user gets the
  // feedback inline instead of round-tripping a 400.
  const termInvalid =
    !!form.placement_date && !!form.maturity_date && form.maturity_date <= form.placement_date;

  function submit(close: () => void) {
    if (!user) return;
    if (!form.risk_profile) return;
    if (termInvalid) return;
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
        bank_name: form.bank_name,
        principal: form.principal,
        interest_rate: form.interest_rate,
        term_months: Number(form.term_months),
        placement_date: form.placement_date,
        maturity_date: form.maturity_date,
        rollover_policy: form.rollover_policy,
        ...(rolledFromInvestmentId ? { rolled_from_investment_id: rolledFromInvestmentId } : {}),
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
        <Button variant={triggerVariant} size={triggerSize}>
          <Plus className="mr-1 size-4" />
          {triggerLabel ?? t("investments:timeDeposit.createTrigger")}
        </Button>
      }
      formClassName="space-y-4"
      title={t("investments:timeDeposit.createTitle")}
      description={t("investments:timeDeposit.createDescription")}
      submitLabel={t("common:actions.create")}
      pendingLabel={t("common:actions.creating")}
      isPending={mutation.isPending}
      tag={{ value: tagId, onChange: setTagId }}
      validate={() =>
        termInvalid ? { td_maturity_date: t("investments:timeDeposit.maturityAfterPlacement") } : {}
      }
      error={mutation.error}
      onSubmit={submit}
      onClosed={() => {
        setForm({ ...emptyForm(), ...prefill });
        setTagId(null);
        mutation.reset();
      }}
    >
      <TimeDepositFormBody
        mode="create"
        form={form}
        setForm={setForm}
        soleOwnerID={effectiveSoleOwnerID}
      />
    </PositionFormDialog>
  );
}
