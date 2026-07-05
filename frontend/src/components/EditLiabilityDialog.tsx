import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useUpdateLiability } from "@/hooks/useLiabilities";
import { useHouseholdMembers } from "@/hooks/useHouseholdMembers";
import { preferredName } from "@/lib/names";
import { useSession } from "@/hooks/useSession";
import { PositionFormDialog } from "@/components/PositionFormDialog";
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
  const { data: members } = useHouseholdMembers();
  const [form, setForm] = useState(() => toForm(liability));

  const effectiveSoleOwnerID = form.sole_owner_user_id ?? user?.id ?? null;

  function submit(close: () => void) {
    mutation.mutate(
      {
        display_name: form.display_name,
        description: form.description || null,
        ownership_type: form.ownership_type,
        sole_owner_user_id:
          form.ownership_type === "sole" ? effectiveSoleOwnerID : null,
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
      contentClassName="max-h-[90vh] overflow-y-auto"
      title={t("liabilities:editTitle")}
      description={t("liabilities:editDescription")}
      submitLabel={t("common:actions.saveChanges")}
      pendingLabel={t("common:actions.saving")}
      isPending={mutation.isPending}
      error={mutation.error}
      onSubmit={submit}
    >
      <div className="grid gap-2">
        <Label htmlFor="edit_l_display_name">
          {t("common:fields.displayName")}
        </Label>
        <Input
          id="edit_l_display_name"
          required
          value={form.display_name}
          onChange={(e) => setForm({ ...form, display_name: e.target.value })}
        />
      </div>

      <div className="grid gap-2">
        <Label htmlFor="edit_l_counterparty">
          {t("liabilities:fields.counterparty")}
        </Label>
        <Input
          id="edit_l_counterparty"
          required
          value={form.counterparty_name}
          onChange={(e) =>
            setForm({ ...form, counterparty_name: e.target.value })
          }
        />
      </div>

      <div className="grid grid-cols-2 gap-3">
        <div className="grid gap-2">
          <Label htmlFor="edit_l_principal">
            {t("liabilities:fields.principalEdit")}
          </Label>
          <Input
            id="edit_l_principal"
            inputMode="decimal"
            value={form.principal}
            onChange={(e) => setForm({ ...form, principal: e.target.value })}
          />
        </div>
        <div className="grid gap-2">
          <Label htmlFor="edit_l_interest_rate">
            {t("liabilities:fields.interestRateEdit")}
          </Label>
          <Input
            id="edit_l_interest_rate"
            inputMode="decimal"
            value={form.interest_rate}
            onChange={(e) =>
              setForm({ ...form, interest_rate: e.target.value })
            }
          />
        </div>
      </div>

      <div className="grid grid-cols-3 gap-3">
        <div className="grid gap-2">
          <Label htmlFor="edit_l_term">
            {t("liabilities:fields.termEdit")}
          </Label>
          <Input
            id="edit_l_term"
            inputMode="numeric"
            value={form.term_months}
            onChange={(e) => setForm({ ...form, term_months: e.target.value })}
          />
        </div>
        <div className="grid gap-2">
          <Label htmlFor="edit_l_start">
            {t("liabilities:fields.startDateEdit")}
          </Label>
          <Input
            id="edit_l_start"
            type="date"
            max="9999-12-31"
            value={form.start_date}
            onChange={(e) => setForm({ ...form, start_date: e.target.value })}
          />
        </div>
        <div className="grid gap-2">
          <Label htmlFor="edit_l_maturity">
            {t("liabilities:fields.maturityDateEdit")}
          </Label>
          <Input
            id="edit_l_maturity"
            type="date"
            max="9999-12-31"
            value={form.maturity_date}
            onChange={(e) =>
              setForm({ ...form, maturity_date: e.target.value })
            }
          />
        </div>
      </div>

      <div className="grid gap-2">
        <Label>{t("common:fields.ownership")}</Label>
        <div className="flex gap-4 text-sm">
          <label className="flex items-center gap-2">
            <input
              type="radio"
              name="edit_l_ownership_type"
              value="joint"
              checked={form.ownership_type === "joint"}
              onChange={() => setForm({ ...form, ownership_type: "joint" })}
            />
            {t("common:ownership.joint")}
          </label>
          <label className="flex items-center gap-2">
            <input
              type="radio"
              name="edit_l_ownership_type"
              value="sole"
              checked={form.ownership_type === "sole"}
              onChange={() => setForm({ ...form, ownership_type: "sole" })}
            />
            {t("common:ownership.soleOwner")}
          </label>
        </div>
        {form.ownership_type === "sole" && (
          <select
            aria-label={t("common:ownership.soleOwner")}
            className="h-9 rounded-md border border-input bg-background px-3 text-sm"
            value={effectiveSoleOwnerID ?? ""}
            onChange={(e) =>
              setForm({ ...form, sole_owner_user_id: e.target.value })
            }
          >
            {(members ?? []).map((m) => (
              <option key={m.id} value={m.id}>
                {preferredName(m)}
                {user && m.id === user.id
                  ? t("common:ownership.youSuffix")
                  : ""}
              </option>
            ))}
          </select>
        )}
      </div>

      <div className="grid gap-2">
        <Label htmlFor="edit_l_description">
          {t("common:fields.description")}
        </Label>
        <Input
          id="edit_l_description"
          value={form.description}
          onChange={(e) => setForm({ ...form, description: e.target.value })}
        />
      </div>
    </PositionFormDialog>
  );
}
