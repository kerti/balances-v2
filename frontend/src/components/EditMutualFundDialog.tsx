import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useUpdateMutualFund } from "@/hooks/useInvestments";
import { useHouseholdMembers } from "@/hooks/useHouseholdMembers";
import { preferredName } from "@/lib/names";
import { useSession } from "@/hooks/useSession";
import { RiskProfileSelect } from "@/components/RiskProfileSelect";
import { MutualFundTypeSelect } from "@/components/MutualFundTypeSelect";
import { PositionFormDialog } from "@/components/PositionFormDialog";
import type { MutualFund, MutualFundListItem } from "@/api/types";

type Props = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  mutualFund: MutualFund | MutualFundListItem;
};

function toForm(m: MutualFund | MutualFundListItem) {
  return {
    display_name: m.investment.display_name,
    description: m.investment.description ?? "",
    ownership_type: m.investment.ownership_type,
    sole_owner_user_id: m.investment.sole_owner_user_id,
    risk_profile: m.investment.risk_profile,
    fund_code: m.details.fund_code,
    fund_manager: m.details.fund_manager ?? "",
    fund_type: m.details.fund_type,
  };
}

export function EditMutualFundDialog({
  open,
  onOpenChange,
  mutualFund,
}: Props) {
  const { t } = useTranslation(["investments", "common"]);
  const mutation = useUpdateMutualFund(mutualFund.investment.id);
  const { data: user } = useSession();
  const { data: members } = useHouseholdMembers();
  const [form, setForm] = useState(() => toForm(mutualFund));

  const effectiveSoleOwnerID = form.sole_owner_user_id ?? user?.id ?? null;

  function submit(close: () => void) {
    mutation.mutate(
      {
        display_name: form.display_name,
        description: form.description || null,
        ownership_type: form.ownership_type,
        sole_owner_user_id:
          form.ownership_type === "sole" ? effectiveSoleOwnerID : null,
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
      contentClassName="max-h-[90vh] overflow-y-auto"
      title={t("investments:mutualFund.editTitle")}
      description={t("investments:mutualFund.editDescription")}
      submitLabel={t("common:actions.saveChanges")}
      pendingLabel={t("common:actions.saving")}
      isPending={mutation.isPending}
      error={mutation.error}
      onSubmit={submit}
    >
      <div className="grid gap-2">
        <Label htmlFor="edit_mf_display_name">
          {t("common:fields.displayName")}
        </Label>
        <Input
          id="edit_mf_display_name"
          required
          value={form.display_name}
          onChange={(e) => setForm({ ...form, display_name: e.target.value })}
        />
      </div>

      <div className="grid grid-cols-2 gap-3">
        <div className="grid gap-2">
          <Label htmlFor="edit_mf_fund_code">
            {t("investments:mutualFund.fields.fundCode")}
          </Label>
          <Input
            id="edit_mf_fund_code"
            required
            value={form.fund_code}
            onChange={(e) => setForm({ ...form, fund_code: e.target.value })}
          />
        </div>
        <div className="grid gap-2">
          <Label htmlFor="edit_mf_fund_manager">
            {t("investments:mutualFund.fields.fundManager")}
          </Label>
          <Input
            id="edit_mf_fund_manager"
            value={form.fund_manager}
            onChange={(e) => setForm({ ...form, fund_manager: e.target.value })}
          />
        </div>
      </div>

      <MutualFundTypeSelect
        idPrefix="mf_edit"
        value={form.fund_type}
        onChange={(v) => setForm({ ...form, fund_type: v })}
      />

      <div className="grid gap-2">
        <Label>{t("common:fields.ownership")}</Label>
        <div className="flex gap-4 text-sm">
          <label className="flex items-center gap-2">
            <input
              type="radio"
              name="edit_mf_ownership_type"
              value="joint"
              checked={form.ownership_type === "joint"}
              onChange={() => setForm({ ...form, ownership_type: "joint" })}
            />
            {t("investments:ownership.joint")}
          </label>
          <label className="flex items-center gap-2">
            <input
              type="radio"
              name="edit_mf_ownership_type"
              value="sole"
              checked={form.ownership_type === "sole"}
              onChange={() => setForm({ ...form, ownership_type: "sole" })}
            />
            {t("investments:ownership.soleOwner")}
          </label>
        </div>
        {form.ownership_type === "sole" && (
          <select
            aria-label={t("investments:ownership.soleOwnerAria")}
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
        <Label htmlFor="edit_mf_description">
          {t("common:fields.description")}
        </Label>
        <Input
          id="edit_mf_description"
          value={form.description}
          onChange={(e) => setForm({ ...form, description: e.target.value })}
        />
      </div>

      <RiskProfileSelect
        idPrefix="mf_edit"
        value={form.risk_profile}
        onChange={(v) => setForm({ ...form, risk_profile: v })}
      />
    </PositionFormDialog>
  );
}
