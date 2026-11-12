import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useUpdateProperty } from "@/hooks/useProperties";
import { useSession } from "@/hooks/useSession";
import { PositionFormDialog } from "@/components/dialogs/PositionFormDialog";
import { PropertyFormBody, type PropertyFormState } from "@/components/dialogs/forms/PropertyForm";
import type { Property } from "@/api/types";

type Props = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  property: Property;
};

function toForm(property: Property): PropertyFormState {
  return {
    display_name: property.asset.display_name,
    description: property.asset.description ?? "",
    ownership_type: property.asset.ownership_type,
    entry_type: property.asset.entry_type,
    sole_owner_user_id: property.asset.sole_owner_user_id,
    native_currency: property.asset.native_currency,
    property_type: property.details.property_type,
    address: property.details.address ?? "",
    acquisition_date: property.details.acquisition_date
      ? property.details.acquisition_date.slice(0, 10)
      : "",
    acquisition_cost: property.details.acquisition_cost ?? "",
    annual_appreciation_rate: property.details.annual_appreciation_rate ?? "",
  };
}

export function EditPropertyDialog({ open, onOpenChange, property }: Props) {
  const { t } = useTranslation(["assets", "common"]);
  const mutation = useUpdateProperty(property.asset.id);
  const { data: user } = useSession();
  const [form, setForm] = useState(() => toForm(property));
  const [tagId, setTagId] = useState<string | null>(() => property.asset.tag_id);

  const effectiveSoleOwnerID = form.sole_owner_user_id ?? user?.id ?? null;

  function submit(close: () => void) {
    mutation.mutate(
      {
        display_name: form.display_name,
        description: form.description || null,
        ownership_type: form.ownership_type,
        entry_type: form.entry_type,
        tag_id: tagId,
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
      tag={{ value: tagId, onChange: setTagId }}
      error={mutation.error}
      onSubmit={submit}
    >
      <PropertyFormBody
        mode="edit"
        form={form}
        setForm={setForm}
        soleOwnerID={effectiveSoleOwnerID}
      />
    </PositionFormDialog>
  );
}
