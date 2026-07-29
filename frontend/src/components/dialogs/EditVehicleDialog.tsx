import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useUpdateVehicle } from "@/hooks/useVehicles";
import { useSession } from "@/hooks/useSession";
import { PositionFormDialog } from "@/components/dialogs/PositionFormDialog";
import { OwnershipField } from "@/components/common/OwnershipField";
import { Select } from "@/components/ui/select";
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

  const [form, setForm] = useState({
    display_name: vehicle.asset.display_name,
    description: vehicle.asset.description ?? "",
    ownership_type: vehicle.asset.ownership_type,
    sole_owner_user_id: vehicle.asset.sole_owner_user_id,
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
      <div className="grid gap-2">
        <Label htmlFor="ev_display_name">{t("common:fields.displayName")}</Label>
        <Input
          id="ev_display_name"
          required
          value={form.display_name}
          onChange={(e) => setForm({ ...form, display_name: e.target.value })}
        />
      </div>

      <div className="grid gap-2">
        <Label htmlFor="ev_type">{t("assets:vehicle.fields.type")}</Label>
        <Select
          id="ev_type"
          value={form.vehicle_type}
          onChange={(e) =>
            setForm({
              ...form,
              vehicle_type: e.target.value as typeof form.vehicle_type,
            })
          }
        >
          <option value="car">{t("assets:vehicle.vehicleTypes.car")}</option>
          <option value="motorcycle">{t("assets:vehicle.vehicleTypes.motorcycle")}</option>
          <option value="other">{t("assets:vehicle.vehicleTypes.other")}</option>
        </Select>
      </div>

      <div className="grid grid-cols-2 gap-3 [&>*]:content-end">
        <div className="grid gap-2">
          <Label htmlFor="ev_make">{t("assets:vehicle.fields.makeEdit")}</Label>
          <Input
            id="ev_make"
            value={form.make}
            onChange={(e) => setForm({ ...form, make: e.target.value })}
          />
        </div>
        <div className="grid gap-2">
          <Label htmlFor="ev_model">{t("assets:vehicle.fields.modelEdit")}</Label>
          <Input
            id="ev_model"
            value={form.model}
            onChange={(e) => setForm({ ...form, model: e.target.value })}
          />
        </div>
      </div>

      <div className="grid grid-cols-2 gap-3 [&>*]:content-end">
        <div className="grid gap-2">
          <Label htmlFor="ev_year">{t("assets:vehicle.fields.yearEdit")}</Label>
          <Input
            id="ev_year"
            type="number"
            value={form.year}
            onChange={(e) => setForm({ ...form, year: e.target.value })}
          />
        </div>
        <div className="grid gap-2">
          <Label htmlFor="ev_plate">{t("assets:vehicle.fields.plateNumberEdit")}</Label>
          <Input
            id="ev_plate"
            value={form.plate_number}
            onChange={(e) => setForm({ ...form, plate_number: e.target.value })}
          />
        </div>
      </div>

      <div className="grid gap-2">
        <Label htmlFor="ev_depr">{t("assets:vehicle.fields.depreciationRateEdit")}</Label>
        <Input
          id="ev_depr"
          inputMode="decimal"
          value={form.annual_depreciation_rate}
          onChange={(e) =>
            setForm({
              ...form,
              annual_depreciation_rate: e.target.value,
            })
          }
        />
      </div>

      <OwnershipField
        idPrefix="vehicle_edit"
        value={form.ownership_type}
        onChange={(v) => setForm({ ...form, ownership_type: v })}
        soleOwnerID={effectiveSoleOwnerID}
        onSoleOwnerChange={(v) => setForm({ ...form, sole_owner_user_id: v })}
      />

      <div className="grid gap-2">
        <Label htmlFor="ev_description">{t("common:fields.description")}</Label>
        <Input
          id="ev_description"
          value={form.description}
          onChange={(e) => setForm({ ...form, description: e.target.value })}
        />
      </div>
    </PositionFormDialog>
  );
}
