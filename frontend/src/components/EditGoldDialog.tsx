import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useUpdateGold, type GoldForm } from "@/hooks/useInvestments";
import { useHouseholdMembers } from "@/hooks/useHouseholdMembers";
import { preferredName } from "@/lib/names";
import { useSession } from "@/hooks/useSession";
import { RiskProfileSelect } from "@/components/RiskProfileSelect";
import { GoldPuritySelect } from "@/components/GoldPuritySelect";
import { PositionFormDialog } from "@/components/PositionFormDialog";
import type { Gold, GoldListItem } from "@/api/types";

type Props = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  gold: Gold | GoldListItem;
};

function toForm(g: Gold | GoldListItem) {
  return {
    display_name: g.investment.display_name,
    description: g.investment.description ?? "",
    ownership_type: g.investment.ownership_type,
    sole_owner_user_id: g.investment.sole_owner_user_id,
    risk_profile: g.investment.risk_profile,
    form: g.details.form as GoldForm,
    purity: g.details.purity,
  };
}

export function EditGoldDialog({ open, onOpenChange, gold }: Props) {
  const { t } = useTranslation(["investments", "common"]);
  const mutation = useUpdateGold(gold.investment.id);
  const { data: user } = useSession();
  const { data: members } = useHouseholdMembers();
  const [form, setForm] = useState(() => toForm(gold));

  const effectiveSoleOwnerID = form.sole_owner_user_id ?? user?.id ?? null;

  function submit(close: () => void) {
    mutation.mutate(
      {
        display_name: form.display_name,
        description: form.description || null,
        ownership_type: form.ownership_type,
        sole_owner_user_id: form.ownership_type === "sole" ? effectiveSoleOwnerID : null,
        risk_profile: form.risk_profile,
        form: form.form,
        purity: form.purity,
      },
      { onSuccess: close },
    );
  }

  return (
    <PositionFormDialog
      open={open}
      onOpenChange={onOpenChange}
      contentClassName="max-h-[90vh] overflow-y-auto"
      title={t("investments:gold.editTitle")}
      description={t("investments:gold.editDescription")}
      submitLabel={t("common:actions.saveChanges")}
      pendingLabel={t("common:actions.saving")}
      isPending={mutation.isPending}
      error={mutation.error}
      onSubmit={submit}
    >
      <div className="grid gap-2">
        <Label htmlFor="edit_gold_display_name">{t("common:fields.displayName")}</Label>
        <Input
          id="edit_gold_display_name"
          required
          value={form.display_name}
          onChange={(e) => setForm({ ...form, display_name: e.target.value })}
        />
      </div>

      <div className="grid grid-cols-2 gap-3">
        <div className="grid gap-2">
          <Label htmlFor="edit_gold_form">{t("investments:gold.fields.form")}</Label>
          <select
            id="edit_gold_form"
            className="h-9 rounded-md border border-input bg-background px-3 text-sm"
            value={form.form}
            onChange={(e) => setForm({ ...form, form: e.target.value as GoldForm })}
          >
            <option value="bar">{t("investments:gold.goldForms.bar")}</option>
            <option value="coin">{t("investments:gold.goldForms.coin")}</option>
            <option value="digital">{t("investments:gold.goldForms.digital")}</option>
            <option value="jewelry">{t("investments:gold.goldForms.jewelry")}</option>
          </select>
        </div>
        <GoldPuritySelect
          idPrefix="gold_edit"
          value={form.purity}
          onChange={(v) => setForm({ ...form, purity: v })}
        />
      </div>

      <div className="grid gap-2">
        <Label>{t("common:fields.ownership")}</Label>
        <div className="flex gap-4 text-sm">
          <label className="flex items-center gap-2">
            <input
              type="radio"
              name="edit_gold_ownership_type"
              value="joint"
              checked={form.ownership_type === "joint"}
              onChange={() => setForm({ ...form, ownership_type: "joint" })}
            />
            {t("investments:ownership.joint")}
          </label>
          <label className="flex items-center gap-2">
            <input
              type="radio"
              name="edit_gold_ownership_type"
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
            onChange={(e) => setForm({ ...form, sole_owner_user_id: e.target.value })}
          >
            {(members ?? []).map((m) => (
              <option key={m.id} value={m.id}>
                {preferredName(m)}
                {user && m.id === user.id ? t("common:ownership.youSuffix") : ""}
              </option>
            ))}
          </select>
        )}
      </div>

      <div className="grid gap-2">
        <Label htmlFor="edit_gold_description">{t("common:fields.description")}</Label>
        <Input
          id="edit_gold_description"
          value={form.description}
          onChange={(e) => setForm({ ...form, description: e.target.value })}
        />
      </div>

      <RiskProfileSelect
        idPrefix="gold_edit"
        value={form.risk_profile}
        onChange={(v) => setForm({ ...form, risk_profile: v })}
      />
    </PositionFormDialog>
  );
}
