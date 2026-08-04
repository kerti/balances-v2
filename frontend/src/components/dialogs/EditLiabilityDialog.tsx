import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useUpdateLiability } from "@/hooks/useLiabilities";
import { useSession } from "@/hooks/useSession";
import { PositionFormDialog } from "@/components/dialogs/PositionFormDialog";
import { OwnershipField } from "@/components/common/OwnershipField";
import { EntryTypeField } from "@/components/common/EntryTypeField";
import type { Liability } from "@/api/types";

type Props = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  liability: Liability;
};

function toForm(l: Liability) {
  return {
    display_name: l.display_name,
    description: l.description ?? "",
    ownership_type: l.ownership_type,
    entry_type: l.entry_type,
    sole_owner_user_id: l.sole_owner_user_id,
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

  const effectiveSoleOwnerID = form.sole_owner_user_id ?? user?.id ?? null;

  function submit(close: () => void) {
    mutation.mutate(
      {
        display_name: form.display_name,
        description: form.description || null,
        ownership_type: form.ownership_type,
        entry_type: form.entry_type,
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
      error={mutation.error}
      onSubmit={submit}
    >
      <div className="grid gap-2">
        <Label htmlFor="edit_l_display_name">{t("common:fields.displayName")}</Label>
        <Input
          id="edit_l_display_name"
          required
          value={form.display_name}
          onChange={(e) => setForm({ ...form, display_name: e.target.value })}
        />
      </div>

      <div className="grid gap-2">
        <Label htmlFor="edit_l_counterparty">{t("liabilities:fields.counterparty")}</Label>
        <Input
          id="edit_l_counterparty"
          required
          value={form.counterparty_name}
          onChange={(e) => setForm({ ...form, counterparty_name: e.target.value })}
        />
      </div>

      <div className="grid grid-cols-2 gap-3 [&>*]:content-end">
        <div className="grid gap-2">
          <Label htmlFor="edit_l_principal">{t("liabilities:fields.principalEdit")}</Label>
          <Input
            id="edit_l_principal"
            inputMode="decimal"
            value={form.principal}
            onChange={(e) => setForm({ ...form, principal: e.target.value })}
          />
        </div>
        <div className="grid gap-2">
          <Label htmlFor="edit_l_interest_rate">{t("liabilities:fields.interestRateEdit")}</Label>
          <Input
            id="edit_l_interest_rate"
            inputMode="decimal"
            value={form.interest_rate}
            onChange={(e) => setForm({ ...form, interest_rate: e.target.value })}
          />
        </div>
      </div>

      {/* Stacks on phones for the same reason as the create dialog's row. */}
      <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
        <div className="grid gap-2">
          <Label htmlFor="edit_l_term">{t("liabilities:fields.termEdit")}</Label>
          <Input
            id="edit_l_term"
            inputMode="numeric"
            value={form.term_months}
            onChange={(e) => setForm({ ...form, term_months: e.target.value })}
          />
        </div>
        <div className="grid gap-2">
          <Label htmlFor="edit_l_start">{t("liabilities:fields.startDateEdit")}</Label>
          <Input
            id="edit_l_start"
            type="date"
            max="9999-12-31"
            value={form.start_date}
            onChange={(e) => setForm({ ...form, start_date: e.target.value })}
          />
        </div>
        <div className="grid gap-2">
          <Label htmlFor="edit_l_maturity">{t("liabilities:fields.maturityDateEdit")}</Label>
          <Input
            id="edit_l_maturity"
            type="date"
            max="9999-12-31"
            value={form.maturity_date}
            onChange={(e) => setForm({ ...form, maturity_date: e.target.value })}
          />
        </div>
      </div>

      <OwnershipField
        idPrefix="liability_edit"
        value={form.ownership_type}
        onChange={(v) => setForm({ ...form, ownership_type: v })}
        soleOwnerID={effectiveSoleOwnerID}
        onSoleOwnerChange={(v) => setForm({ ...form, sole_owner_user_id: v })}
      />

      <EntryTypeField
        idPrefix="liability_edit"
        group="liability"
        value={form.entry_type}
        onChange={(v) => setForm({ ...form, entry_type: v })}
      />

      <div className="grid gap-2">
        <Label htmlFor="edit_l_description">{t("common:fields.description")}</Label>
        <Input
          id="edit_l_description"
          value={form.description}
          onChange={(e) => setForm({ ...form, description: e.target.value })}
        />
      </div>
    </PositionFormDialog>
  );
}
