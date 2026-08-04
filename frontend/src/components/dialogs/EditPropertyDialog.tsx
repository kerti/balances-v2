import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useUpdateProperty } from "@/hooks/useProperties";
import { useSession } from "@/hooks/useSession";
import { PositionFormDialog } from "@/components/dialogs/PositionFormDialog";
import { OwnershipField } from "@/components/common/OwnershipField";
import { EntryTypeField } from "@/components/common/EntryTypeField";
import { Select } from "@/components/ui/select";
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

  const [form, setForm] = useState({
    display_name: property.asset.display_name,
    description: property.asset.description ?? "",
    ownership_type: property.asset.ownership_type,
    entry_type: property.asset.entry_type,
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
        entry_type: form.entry_type,
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
        <Select
          id="edit_p_type"
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
        </Select>
      </div>

      <div className="grid gap-2">
        <Label htmlFor="edit_p_address">{t("assets:property.fields.addressEdit")}</Label>
        <Input
          id="edit_p_address"
          value={form.address}
          onChange={(e) => setForm({ ...form, address: e.target.value })}
        />
      </div>

      <div className="grid grid-cols-2 gap-3 [&>*]:content-end">
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

      <OwnershipField
        idPrefix="property_edit"
        value={form.ownership_type}
        onChange={(v) => setForm({ ...form, ownership_type: v })}
        soleOwnerID={effectiveSoleOwnerID}
        onSoleOwnerChange={(v) => setForm({ ...form, sole_owner_user_id: v })}
      />

      <EntryTypeField
        idPrefix="property_edit"
        group="asset"
        value={form.entry_type}
        onChange={(v) => setForm({ ...form, entry_type: v })}
      />

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
