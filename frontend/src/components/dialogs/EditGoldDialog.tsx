import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useUpdateGold, type GoldForm } from "@/hooks/useInvestments";
import { useSession } from "@/hooks/useSession";
import { RiskProfileSelect } from "@/components/common/RiskProfileSelect";
import { GoldPuritySelect } from "@/components/common/GoldPuritySelect";
import { PositionFormDialog } from "@/components/dialogs/PositionFormDialog";
import { OwnershipField } from "@/components/common/OwnershipField";
import { Select } from "@/components/ui/select";
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

      <div className="grid grid-cols-2 gap-3 [&>*]:content-end">
        <div className="grid gap-2">
          <Label htmlFor="edit_gold_form">{t("investments:gold.fields.form")}</Label>
          <Select
            id="edit_gold_form"
            value={form.form}
            onChange={(e) => setForm({ ...form, form: e.target.value as GoldForm })}
          >
            <option value="bar">{t("investments:gold.goldForms.bar")}</option>
            <option value="coin">{t("investments:gold.goldForms.coin")}</option>
            <option value="digital">{t("investments:gold.goldForms.digital")}</option>
            <option value="jewelry">{t("investments:gold.goldForms.jewelry")}</option>
          </Select>
        </div>
        <GoldPuritySelect
          idPrefix="gold_edit"
          value={form.purity}
          onChange={(v) => setForm({ ...form, purity: v })}
        />
      </div>

      <OwnershipField
        idPrefix="gold_edit"
        value={form.ownership_type}
        onChange={(v) => setForm({ ...form, ownership_type: v })}
        soleOwnerID={effectiveSoleOwnerID}
        onSoleOwnerChange={(v) => setForm({ ...form, sole_owner_user_id: v })}
      />

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
