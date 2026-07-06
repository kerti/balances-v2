import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useCreateGold, type GoldForm } from "@/hooks/useInvestments";
import { useSession } from "@/hooks/useSession";
import { useHouseholdMembers } from "@/hooks/useHouseholdMembers";
import { preferredName } from "@/lib/names";
import { RiskProfileSelect } from "@/components/RiskProfileSelect";
import { GoldPuritySelect } from "@/components/GoldPuritySelect";
import { PositionFormDialog } from "@/components/PositionFormDialog";
import type { RiskProfile } from "@/api/types";

function emptyForm() {
  return {
    display_name: "",
    description: "",
    ownership_type: "joint" as "sole" | "joint",
    sole_owner_user_id: null as string | null,
    risk_profile: "" as RiskProfile | "",
    native_currency: "IDR",
    form: "bar" as GoldForm,
    purity: "0.9999",
  };
}

export function CreateGoldDialog() {
  const { t } = useTranslation(["investments", "common"]);
  const [form, setForm] = useState(emptyForm);
  const { data: user } = useSession();
  const { data: members } = useHouseholdMembers();
  const mutation = useCreateGold();

  const effectiveSoleOwnerID = form.sole_owner_user_id ?? user?.id ?? null;

  function submit(close: () => void) {
    if (!user) return;
    if (!form.risk_profile) return;
    mutation.mutate(
      {
        display_name: form.display_name,
        description: form.description || null,
        ownership_type: form.ownership_type,
        sole_owner_user_id: form.ownership_type === "sole" ? effectiveSoleOwnerID : null,
        risk_profile: form.risk_profile,
        native_currency: form.native_currency,
        form: form.form,
        purity: form.purity,
      },
      { onSuccess: close },
    );
  }

  return (
    <PositionFormDialog
      trigger={
        <Button>
          <Plus className="mr-1 size-4" />
          {t("investments:gold.createTrigger")}
        </Button>
      }
      contentClassName="max-h-[90vh] overflow-y-auto"
      title={t("investments:gold.createTitle")}
      description={t("investments:gold.createDescription")}
      submitLabel={t("common:actions.create")}
      pendingLabel={t("common:actions.creating")}
      isPending={mutation.isPending}
      error={mutation.error}
      onSubmit={submit}
      onClosed={() => {
        setForm(emptyForm());
        mutation.reset();
      }}
    >
      <div className="grid gap-2">
        <Label htmlFor="gold_display_name">{t("common:fields.displayName")}</Label>
        <Input
          id="gold_display_name"
          required
          value={form.display_name}
          onChange={(e) => setForm({ ...form, display_name: e.target.value })}
          placeholder={t("investments:gold.placeholders.displayName")}
        />
      </div>

      <div className="grid grid-cols-2 gap-3">
        <div className="grid gap-2">
          <Label htmlFor="gold_form">{t("investments:gold.fields.form")}</Label>
          <select
            id="gold_form"
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
          idPrefix="gold_create"
          value={form.purity}
          onChange={(v) => setForm({ ...form, purity: v })}
        />
      </div>

      <div className="grid gap-2">
        <Label htmlFor="gold_currency">{t("common:fields.currency")}</Label>
        <Input
          id="gold_currency"
          required
          value={form.native_currency}
          onChange={(e) =>
            setForm({
              ...form,
              native_currency: e.target.value.toUpperCase(),
            })
          }
          placeholder={t("investments:gold.placeholders.currency")}
          maxLength={3}
        />
      </div>

      <div className="grid gap-2">
        <Label>{t("common:fields.ownership")}</Label>
        <div className="flex gap-4 text-sm">
          <label className="flex items-center gap-2">
            <input
              type="radio"
              name="gold_ownership_type"
              value="joint"
              checked={form.ownership_type === "joint"}
              onChange={() => setForm({ ...form, ownership_type: "joint" })}
            />
            {t("investments:ownership.joint")}
          </label>
          <label className="flex items-center gap-2">
            <input
              type="radio"
              name="gold_ownership_type"
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
        <Label htmlFor="gold_description">{t("common:fields.description")}</Label>
        <Input
          id="gold_description"
          value={form.description}
          onChange={(e) => setForm({ ...form, description: e.target.value })}
        />
      </div>

      <RiskProfileSelect
        idPrefix="gold_create"
        value={form.risk_profile}
        onChange={(v) => setForm({ ...form, risk_profile: v })}
      />
    </PositionFormDialog>
  );
}
