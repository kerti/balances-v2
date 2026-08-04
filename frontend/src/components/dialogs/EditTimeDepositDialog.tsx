import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useUpdateTimeDeposit } from "@/hooks/useInvestments";
import { useSession } from "@/hooks/useSession";
import { RiskProfileSelect } from "@/components/common/RiskProfileSelect";
import { PositionFormDialog } from "@/components/dialogs/PositionFormDialog";
import { OwnershipField } from "@/components/common/OwnershipField";
import { EntryTypeField } from "@/components/common/EntryTypeField";
import { Select } from "@/components/ui/select";
import type { RolloverPolicy, TimeDeposit, TimeDepositListItem } from "@/api/types";

type Props = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  timeDeposit: TimeDeposit | TimeDepositListItem;
};

function toForm(td: TimeDeposit | TimeDepositListItem) {
  const i = td.investment;
  const d = td.details;
  return {
    display_name: i.display_name,
    description: i.description ?? "",
    ownership_type: i.ownership_type,
    entry_type: i.entry_type,
    sole_owner_user_id: i.sole_owner_user_id,
    risk_profile: td.investment.risk_profile,
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
      submitDisabled={termInvalid}
      error={mutation.error}
      onSubmit={submit}
    >
      <div className="space-y-3">
        <div className="grid gap-2">
          <Label htmlFor="edit_td_display_name">{t("common:fields.displayName")}</Label>
          <Input
            id="edit_td_display_name"
            required
            value={form.display_name}
            onChange={(e) => setForm({ ...form, display_name: e.target.value })}
          />
        </div>
        <div className="grid gap-2">
          <Label htmlFor="edit_td_description">{t("common:fields.description")}</Label>
          <Input
            id="edit_td_description"
            value={form.description}
            onChange={(e) => setForm({ ...form, description: e.target.value })}
          />
        </div>
      </div>

      <div className="space-y-3 border-t pt-4">
        <div className="grid gap-2">
          <Label htmlFor="edit_td_bank_name">{t("investments:timeDeposit.fields.bankName")}</Label>
          <Input
            id="edit_td_bank_name"
            required
            value={form.bank_name}
            onChange={(e) => setForm({ ...form, bank_name: e.target.value })}
          />
        </div>
        <div className="grid gap-2">
          <Label htmlFor="edit_td_principal">{t("investments:timeDeposit.fields.principal")}</Label>
          <Input
            id="edit_td_principal"
            required
            inputMode="decimal"
            value={form.principal}
            onChange={(e) => setForm({ ...form, principal: e.target.value })}
          />
        </div>
      </div>

      <div className="space-y-3 border-t pt-4">
        <div className="grid grid-cols-2 gap-3 [&>*]:content-end">
          <div className="grid gap-2">
            <Label htmlFor="edit_td_interest_rate">
              {t("investments:timeDeposit.fields.interestRate")}
            </Label>
            <Input
              id="edit_td_interest_rate"
              required
              inputMode="decimal"
              value={form.interest_rate}
              onChange={(e) => setForm({ ...form, interest_rate: e.target.value })}
            />
          </div>
          <div className="grid gap-2">
            <Label htmlFor="edit_td_term_months">
              {t("investments:timeDeposit.fields.termMonths")}
            </Label>
            <Input
              id="edit_td_term_months"
              required
              inputMode="numeric"
              value={form.term_months}
              onChange={(e) => setForm({ ...form, term_months: e.target.value })}
            />
          </div>
        </div>
        <div className="grid grid-cols-2 gap-3 [&>*]:content-end">
          <div className="grid gap-2">
            <Label htmlFor="edit_td_placement_date">
              {t("investments:timeDeposit.fields.placementDate")}
            </Label>
            <Input
              id="edit_td_placement_date"
              required
              type="date"
              max="9999-12-31"
              value={form.placement_date}
              onChange={(e) => setForm({ ...form, placement_date: e.target.value })}
            />
          </div>
          <div className="grid gap-2">
            <Label htmlFor="edit_td_maturity_date">
              {t("investments:timeDeposit.fields.maturityDate")}
            </Label>
            <Input
              id="edit_td_maturity_date"
              required
              type="date"
              min={form.placement_date || undefined}
              max="9999-12-31"
              value={form.maturity_date}
              onChange={(e) => setForm({ ...form, maturity_date: e.target.value })}
            />
          </div>
        </div>
        {termInvalid && (
          <p data-testid="edit-td-term-error" className="text-sm text-destructive">
            {t("investments:timeDeposit.maturityAfterPlacement")}
          </p>
        )}
      </div>

      <div className="space-y-3 border-t pt-4">
        <div className="grid gap-2">
          <Label htmlFor="edit_td_rollover_policy">
            {t("investments:timeDeposit.fields.rolloverPolicy")}
          </Label>
          <Select
            id="edit_td_rollover_policy"
            value={form.rollover_policy}
            onChange={(e) =>
              setForm({
                ...form,
                rollover_policy: e.target.value as RolloverPolicy,
              })
            }
          >
            <option value="auto_renew_principal">
              {t("investments:timeDeposit.rolloverPolicy.auto_renew_principal")}
            </option>
            <option value="auto_renew_with_interest">
              {t("investments:timeDeposit.rolloverPolicy.auto_renew_with_interest")}
            </option>
            <option value="no_rollover">
              {t("investments:timeDeposit.rolloverPolicy.no_rollover")}
            </option>
          </Select>
        </div>
      </div>

      <div className="space-y-3 border-t pt-4">
        <OwnershipField
          idPrefix="td_edit"
          value={form.ownership_type}
          onChange={(v) => setForm({ ...form, ownership_type: v })}
          soleOwnerID={effectiveSoleOwnerID}
          onSoleOwnerChange={(v) => setForm({ ...form, sole_owner_user_id: v })}
        />

        <EntryTypeField
          idPrefix="td_edit"
          group="investment"
          value={form.entry_type}
          onChange={(v) => setForm({ ...form, entry_type: v })}
        />
      </div>

      <RiskProfileSelect
        idPrefix="td_edit"
        value={form.risk_profile}
        onChange={(v) => setForm({ ...form, risk_profile: v })}
      />
    </PositionFormDialog>
  );
}
