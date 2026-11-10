import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useUpdateVehicle } from "@/hooks/useVehicles";
import { useSession } from "@/hooks/useSession";
import { PositionFormDialog } from "@/components/dialogs/PositionFormDialog";
import { VehicleFormBody, type VehicleFormState } from "@/components/dialogs/forms/VehicleForm";
import type { Vehicle } from "@/api/types";

type Props = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  vehicle: Vehicle;
};

export function EditVehicleDialog({ open, onOpenChange, vehicle }: Props) {
  const { t } = useTranslation(["assets", "common"]);
  const mutation = useUpdateVehicle(vehicle.asset.id);
  const { data: user } = useSession();

  const [form, setForm] = useState<VehicleFormState>({
    display_name: vehicle.asset.display_name,
    description: vehicle.asset.description ?? "",
    ownership_type: vehicle.asset.ownership_type,
    entry_type: vehicle.asset.entry_type,
    sole_owner_user_id: vehicle.asset.sole_owner_user_id,
    native_currency: vehicle.asset.native_currency,
    vehicle_type: vehicle.details.vehicle_type,
    make: vehicle.details.make ?? "",
    model: vehicle.details.model ?? "",
    year: vehicle.details.year ? String(vehicle.details.year) : "",
    plate_number: vehicle.details.plate_number ?? "",
    annual_depreciation_rate: vehicle.details.annual_depreciation_rate ?? "",
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
        vehicle_type: form.vehicle_type,
        make: form.make || null,
        model: form.model || null,
        year: form.year ? Number(form.year) : null,
        plate_number: form.plate_number || null,
        annual_depreciation_rate: form.annual_depreciation_rate || null,
      },
      { onSuccess: close },
    );
  }

  return (
    <PositionFormDialog
      open={open}
      onOpenChange={onOpenChange}
      title={t("assets:vehicle.editTitle")}
      description={t("assets:vehicle.editDescription")}
      submitLabel={t("common:actions.saveChanges")}
      pendingLabel={t("common:actions.saving")}
      isPending={mutation.isPending}
      error={mutation.error}
      onSubmit={submit}
    >
      <VehicleFormBody
        mode="edit"
        form={form}
        setForm={setForm}
        soleOwnerID={effectiveSoleOwnerID}
      />
    </PositionFormDialog>
  );
}
