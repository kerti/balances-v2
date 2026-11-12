import { useTranslation } from "react-i18next";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import { NumericInput } from "@/components/common/NumericInput";
import { OwnershipField } from "@/components/common/OwnershipField";
import { EntryTypeField } from "@/components/common/EntryTypeField";
import { RATE_DECIMALS, RATE_INTEGER_DIGITS } from "@/lib/format";
import type { EntryType } from "@/api/types";
import type { FormMode } from "./mode";
import { LockedFieldHint } from "./LockedFieldHint";

export type VehicleFormState = {
  display_name: string;
  description: string;
  ownership_type: "sole" | "joint";
  entry_type: EntryType;
  sole_owner_user_id: string | null;
  native_currency: string;
  vehicle_type: "car" | "motorcycle" | "other";
  make: string;
  model: string;
  year: string;
  plate_number: string;
  annual_depreciation_rate: string;
};

type Props = {
  mode: FormMode;
  form: VehicleFormState;
  setForm: (next: VehicleFormState) => void;
  soleOwnerID: string | null;
};

export function VehicleFormBody({ mode, form, setForm, soleOwnerID }: Props) {
  const { t } = useTranslation(["assets", "common"]);
  return (
    <>
      <div className="grid gap-2">
        <Label htmlFor="v_display_name">{t("common:fields.displayName")}</Label>
        <Input
          id="v_display_name"
          required
          value={form.display_name}
          onChange={(e) => setForm({ ...form, display_name: e.target.value })}
          placeholder={t("assets:vehicle.placeholders.displayName")}
        />
      </div>

      <div className="grid grid-cols-2 gap-3 [&>*]:content-end">
        <div className="grid gap-2">
          <Label htmlFor="v_type">{t("assets:vehicle.fields.type")}</Label>
          <Select
            id="v_type"
            value={form.vehicle_type}
            onChange={(e) =>
              setForm({ ...form, vehicle_type: e.target.value as typeof form.vehicle_type })
            }
          >
            <option value="car">{t("assets:vehicle.vehicleTypes.car")}</option>
            <option value="motorcycle">{t("assets:vehicle.vehicleTypes.motorcycle")}</option>
            <option value="other">{t("assets:vehicle.vehicleTypes.other")}</option>
          </Select>
        </div>
        <div className="grid gap-2">
          <Label htmlFor="v_native_currency">{t("common:fields.currency")}</Label>
          <LockedFieldHint mode={mode} id="v_native_currency" />
          <Input
            id="v_native_currency"
            required
            disabled={mode === "edit"}
            value={form.native_currency}
            onChange={(e) => setForm({ ...form, native_currency: e.target.value.toUpperCase() })}
            placeholder={t("assets:vehicle.placeholders.currency")}
            maxLength={3}
          />
        </div>
      </div>

      <div className="grid grid-cols-2 gap-3 [&>*]:content-end">
        <div className="grid gap-2">
          <Label htmlFor="v_make">{t("assets:vehicle.fields.make")}</Label>
          <Input
            id="v_make"
            value={form.make}
            onChange={(e) => setForm({ ...form, make: e.target.value })}
            placeholder={t("assets:vehicle.placeholders.make")}
          />
        </div>
        <div className="grid gap-2">
          <Label htmlFor="v_model">{t("assets:vehicle.fields.model")}</Label>
          <Input
            id="v_model"
            value={form.model}
            onChange={(e) => setForm({ ...form, model: e.target.value })}
            placeholder={t("assets:vehicle.placeholders.model")}
          />
        </div>
      </div>

      <div className="grid grid-cols-2 gap-3 [&>*]:content-end">
        <div className="grid gap-2">
          <Label htmlFor="v_year">{t("assets:vehicle.fields.year")}</Label>
          <Input
            id="v_year"
            type="number"
            value={form.year}
            onChange={(e) => setForm({ ...form, year: e.target.value })}
            placeholder={t("assets:vehicle.placeholders.year")}
          />
        </div>
        <div className="grid gap-2">
          <Label htmlFor="v_plate">{t("assets:vehicle.fields.plateNumber")}</Label>
          <Input
            id="v_plate"
            value={form.plate_number}
            onChange={(e) => setForm({ ...form, plate_number: e.target.value })}
          />
        </div>
      </div>

      <div className="grid gap-2">
        <Label htmlFor="v_depr">{t("assets:vehicle.fields.depreciationRate")}</Label>
        <NumericInput
          id="v_depr"
          decimals={RATE_DECIMALS}
          maxIntegerDigits={RATE_INTEGER_DIGITS}
          value={form.annual_depreciation_rate}
          onChange={(v) => setForm({ ...form, annual_depreciation_rate: v })}
          placeholder={t("assets:vehicle.placeholders.depreciationRate")}
        />
      </div>

      <OwnershipField
        idPrefix={`vehicle_${mode}`}
        value={form.ownership_type}
        onChange={(v) => setForm({ ...form, ownership_type: v })}
        soleOwnerID={soleOwnerID}
        onSoleOwnerChange={(v) => setForm({ ...form, sole_owner_user_id: v })}
      />

      <EntryTypeField
        idPrefix={`vehicle_${mode}`}
        group="asset"
        value={form.entry_type}
        onChange={(v) => setForm({ ...form, entry_type: v })}
      />

      <div className="grid gap-2">
        <Label htmlFor="v_description">{t("common:fields.description")}</Label>
        <Input
          id="v_description"
          value={form.description}
          onChange={(e) => setForm({ ...form, description: e.target.value })}
        />
      </div>
    </>
  );
}
