import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useUpdateProperty } from "@/hooks/useProperties";
import { useHouseholdMembers } from "@/hooks/useHouseholdMembers";
import { preferredName } from "@/lib/names";
import { useSession } from "@/hooks/useSession";
import { PositionFormDialog } from "@/components/PositionFormDialog";
import type { Property } from "@/api/types";

type Props = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  property: Property;
};

export function EditPropertyDialog({ open, onOpenChange, property }: Props) {
  const { t } = useTranslation(["assets", "common"]);
  const mutation = useUpdateProperty(property.asset.id);
  const { data: user } = useSession();
  const { data: members } = useHouseholdMembers();

  const [form, setForm] = useState({
    display_name: property.asset.display_name,
    description: property.asset.description ?? "",
    ownership_type: property.asset.ownership_type,
    sole_owner_user_id: property.asset.sole_owner_user_id,
    property_type: property.details.property_type,
    address: property.details.address ?? "",
    acquisition_date: property.details.acquisition_date
      ? property.details.acquisition_date.slice(0, 10)
      : "",
    acquisition_cost: property.details.acquisition_cost ?? "",
    annual_appreciation_rate: property.details.annual_appreciation_rate ?? "",
  });

  const effectiveSoleOwnerID = form.sole_owner_user_id ?? user?.id ?? null;

  function submit(close: () => void) {
    mutation.mutate(
      {
        display_name: form.display_name,
        description: form.description || null,
        ownership_type: form.ownership_type,
        sole_owner_user_id: form.ownership_type === "sole" ? effectiveSoleOwnerID : null,
        property_type: form.property_type,
        address: form.address || null,
        acquisition_date: form.acquisition_date || null,
        acquisition_cost: form.acquisition_cost || null,
        annual_appreciation_rate: form.annual_appreciation_rate || null,
      },
      { onSuccess: close },
    );
  }

  return (
    <PositionFormDialog
      open={open}
      onOpenChange={onOpenChange}
      title={t("assets:property.editTitle")}
      description={t("assets:property.editDescription")}
      submitLabel={t("common:actions.saveChanges")}
      pendingLabel={t("common:actions.saving")}
      isPending={mutation.isPending}
      error={mutation.error}
      onSubmit={submit}
    >
      <div className="grid gap-2">
        <Label htmlFor="edit_p_display_name">{t("common:fields.displayName")}</Label>
        <Input
          id="edit_p_display_name"
          required
          value={form.display_name}
          onChange={(e) => setForm({ ...form, display_name: e.target.value })}
        />
      </div>

      <div className="grid gap-2">
        <Label htmlFor="edit_p_type">{t("assets:property.fields.type")}</Label>
        <select
          id="edit_p_type"
          className="h-9 rounded-md border border-input bg-background px-3 text-sm"
          value={form.property_type}
          onChange={(e) =>
            setForm({
              ...form,
              property_type: e.target.value as typeof form.property_type,
            })
          }
        >
          <option value="house">{t("assets:property.propertyTypes.house")}</option>
          <option value="apartment">{t("assets:property.propertyTypes.apartment")}</option>
          <option value="land">{t("assets:property.propertyTypes.land")}</option>
          <option value="commercial">{t("assets:property.propertyTypes.commercial")}</option>
        </select>
      </div>

      <div className="grid gap-2">
        <Label htmlFor="edit_p_address">{t("assets:property.fields.addressEdit")}</Label>
        <Input
          id="edit_p_address"
          value={form.address}
          onChange={(e) => setForm({ ...form, address: e.target.value })}
        />
      </div>

      <div className="grid grid-cols-2 gap-3">
        <div className="grid gap-2">
          <Label htmlFor="edit_p_acq_date">{t("assets:property.fields.acquisitionDateEdit")}</Label>
          <Input
            id="edit_p_acq_date"
            type="date"
            max="9999-12-31"
            value={form.acquisition_date}
            onChange={(e) => setForm({ ...form, acquisition_date: e.target.value })}
          />
        </div>
        <div className="grid gap-2">
          <Label htmlFor="edit_p_acq_cost">{t("assets:property.fields.acquisitionCostEdit")}</Label>
          <Input
            id="edit_p_acq_cost"
            inputMode="decimal"
            value={form.acquisition_cost}
            onChange={(e) => setForm({ ...form, acquisition_cost: e.target.value })}
          />
        </div>
      </div>

      <div className="grid gap-2">
        <Label htmlFor="edit_p_apprec">{t("assets:property.fields.appreciationRateEdit")}</Label>
        <Input
          id="edit_p_apprec"
          inputMode="decimal"
          value={form.annual_appreciation_rate}
          onChange={(e) =>
            setForm({
              ...form,
              annual_appreciation_rate: e.target.value,
            })
          }
          placeholder={t("assets:property.placeholders.appreciationRate")}
        />
      </div>

      <div className="grid gap-2">
        <Label>{t("common:fields.ownership")}</Label>
        <div className="flex gap-4 text-sm">
          <label className="flex items-center gap-2">
            <input
              type="radio"
              name="edit_p_ownership_type"
              value="joint"
              checked={form.ownership_type === "joint"}
              onChange={() => setForm({ ...form, ownership_type: "joint" })}
            />
            {t("common:ownership.joint")}
          </label>
          <label className="flex items-center gap-2">
            <input
              type="radio"
              name="edit_p_ownership_type"
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
        <Label htmlFor="edit_p_description">{t("common:fields.description")}</Label>
        <Input
          id="edit_p_description"
          value={form.description}
          onChange={(e) => setForm({ ...form, description: e.target.value })}
        />
      </div>
    </PositionFormDialog>
  );
}
