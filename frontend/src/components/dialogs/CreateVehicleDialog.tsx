import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useCreateVehicle } from "@/hooks/useVehicles";
import { useSession } from "@/hooks/useSession";
import { PositionFormDialog } from "@/components/dialogs/PositionFormDialog";
import { VehicleFormBody, type VehicleFormState } from "@/components/dialogs/forms/VehicleForm";

const empty: VehicleFormState = {
  display_name: "",
  description: "",
  ownership_type: "sole",
  entry_type: "acquired",
  sole_owner_user_id: null,
  native_currency: "IDR",
  vehicle_type: "car",
  make: "",
  model: "",
  year: "",
  plate_number: "",
  annual_depreciation_rate: "",
};

type Props = {
  // Called with the new position's id after a successful create, so the list
  // can land on its detail page (#843).
  onCreated?: (id: string) => void;
};

export function CreateVehicleDialog({ onCreated }: Props) {
  const { t } = useTranslation(["assets", "common"]);
  const [form, setForm] = useState(empty);
  const { data: user } = useSession();
  const mutation = useCreateVehicle();

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
        vehicle_type: form.vehicle_type,
        make: form.make || null,
        model: form.model || null,
        year: form.year ? Number(form.year) : null,
        plate_number: form.plate_number || null,
        annual_depreciation_rate: form.annual_depreciation_rate || null,
      },
      {
        onSuccess: (created) => {
          close();
          onCreated?.(created.asset.id);
        },
      },
    );
  }

  return (
    <PositionFormDialog
      trigger={
        <Button>
          <Plus className="mr-1 size-4" />
          {t("assets:vehicle.createTrigger")}
        </Button>
      }
      title={t("assets:vehicle.createTitle")}
      description={t("assets:vehicle.createDescription")}
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
      <VehicleFormBody
        mode="create"
        form={form}
        setForm={setForm}
        soleOwnerID={effectiveSoleOwnerID}
      />
    </PositionFormDialog>
  );
}
