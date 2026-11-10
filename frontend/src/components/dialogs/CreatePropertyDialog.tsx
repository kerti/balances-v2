import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useCreateProperty } from "@/hooks/useProperties";
import { useSession } from "@/hooks/useSession";
import { PositionFormDialog } from "@/components/dialogs/PositionFormDialog";
import { PropertyFormBody, type PropertyFormState } from "@/components/dialogs/forms/PropertyForm";

const empty: PropertyFormState = {
  display_name: "",
  description: "",
  ownership_type: "joint",
  entry_type: "acquired",
  sole_owner_user_id: null,
  native_currency: "IDR",
  property_type: "house",
  address: "",
  acquisition_date: "",
  acquisition_cost: "",
  annual_appreciation_rate: "",
};

export function CreatePropertyDialog() {
  const { t } = useTranslation(["assets", "common"]);
  const [form, setForm] = useState(empty);
  const { data: user } = useSession();
  const mutation = useCreateProperty();

  const effectiveSoleOwnerID = form.sole_owner_user_id ?? user?.id ?? null;

  function submit(close: () => void) {
    if (!user) return;
    mutation.mutate(
      {
        display_name: form.display_name,
        description: form.description || null,
        ownership_type: form.ownership_type,
        entry_type: form.entry_type,
        sole_owner_user_id: form.ownership_type === "sole" ? effectiveSoleOwnerID : null,
        native_currency: form.native_currency,
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
      trigger={
        <Button>
          <Plus className="mr-1 size-4" />
          {t("assets:property.createTrigger")}
        </Button>
      }
      title={t("assets:property.createTitle")}
      description={t("assets:property.createDescription")}
      submitLabel={t("common:actions.create")}
      pendingLabel={t("common:actions.creating")}
      isPending={mutation.isPending}
      error={mutation.error}
      onSubmit={submit}
      onClosed={() => {
        setForm(empty);
        mutation.reset();
      }}
    >
      <PropertyFormBody
        mode="create"
        form={form}
        setForm={setForm}
        soleOwnerID={effectiveSoleOwnerID}
      />
    </PositionFormDialog>
  );
}
