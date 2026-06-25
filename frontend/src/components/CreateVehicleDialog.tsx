import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useCreateVehicle } from "@/hooks/useVehicles";
import { useSession } from "@/hooks/useSession";
import { useHouseholdMembers } from "@/hooks/useHouseholdMembers";
import { preferredName } from "@/lib/names";
import { errorMessage } from "@/lib/errorMessage";

const empty = {
  display_name: "",
  description: "",
  ownership_type: "joint" as "sole" | "joint",
  sole_owner_user_id: null as string | null,
  native_currency: "IDR",
  vehicle_type: "car" as "car" | "motorcycle" | "other",
  make: "",
  model: "",
  year: "",
  plate_number: "",
  annual_depreciation_rate: "",
};

export function CreateVehicleDialog() {
  const { t } = useTranslation(["assets", "common"]);
  const [open, setOpen] = useState(false);
  const [form, setForm] = useState(empty);
  const { data: user } = useSession();
  const { data: members } = useHouseholdMembers();
  const mutation = useCreateVehicle();

  const effectiveSoleOwnerID = form.sole_owner_user_id ?? user?.id ?? null;

  function close() {
    setOpen(false);
    setForm(empty);
    mutation.reset();
  }

  function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!user) return;
    mutation.mutate(
      {
        display_name: form.display_name,
        description: form.description || null,
        ownership_type: form.ownership_type,
        sole_owner_user_id:
          form.ownership_type === "sole" ? effectiveSoleOwnerID : null,
        native_currency: form.native_currency,
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
    <Dialog open={open} onOpenChange={(o) => (o ? setOpen(true) : close())}>
      <DialogTrigger asChild>
        <Button>
          <Plus className="mr-1 size-4" />
          {t("assets:vehicle.createTrigger")}
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("assets:vehicle.createTitle")}</DialogTitle>
          <DialogDescription>
            {t("assets:vehicle.createDescription")}
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={submit} className="space-y-3">
          <div className="grid gap-2">
            <Label htmlFor="v_display_name">
              {t("common:fields.displayName")}
            </Label>
            <Input
              id="v_display_name"
              required
              value={form.display_name}
              onChange={(e) =>
                setForm({ ...form, display_name: e.target.value })
              }
              placeholder={t("assets:vehicle.placeholders.displayName")}
            />
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div className="grid gap-2">
              <Label htmlFor="v_type">{t("assets:vehicle.fields.type")}</Label>
              <select
                id="v_type"
                className="h-9 rounded-md border border-input bg-background px-3 text-sm"
                value={form.vehicle_type}
                onChange={(e) =>
                  setForm({
                    ...form,
                    vehicle_type: e.target.value as typeof form.vehicle_type,
                  })
                }
              >
                <option value="car">
                  {t("assets:vehicle.vehicleTypes.car")}
                </option>
                <option value="motorcycle">
                  {t("assets:vehicle.vehicleTypes.motorcycle")}
                </option>
                <option value="other">
                  {t("assets:vehicle.vehicleTypes.other")}
                </option>
              </select>
            </div>
            <div className="grid gap-2">
              <Label htmlFor="v_native_currency">
                {t("common:fields.currency")}
              </Label>
              <Input
                id="v_native_currency"
                required
                value={form.native_currency}
                onChange={(e) =>
                  setForm({
                    ...form,
                    native_currency: e.target.value.toUpperCase(),
                  })
                }
                placeholder={t("assets:vehicle.placeholders.currency")}
                maxLength={3}
              />
            </div>
          </div>

          <div className="grid grid-cols-2 gap-3">
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
              <Label htmlFor="v_model">
                {t("assets:vehicle.fields.model")}
              </Label>
              <Input
                id="v_model"
                value={form.model}
                onChange={(e) => setForm({ ...form, model: e.target.value })}
                placeholder={t("assets:vehicle.placeholders.model")}
              />
            </div>
          </div>

          <div className="grid grid-cols-2 gap-3">
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
              <Label htmlFor="v_plate">
                {t("assets:vehicle.fields.plateNumber")}
              </Label>
              <Input
                id="v_plate"
                value={form.plate_number}
                onChange={(e) =>
                  setForm({ ...form, plate_number: e.target.value })
                }
              />
            </div>
          </div>

          <div className="grid gap-2">
            <Label htmlFor="v_depr">
              {t("assets:vehicle.fields.depreciationRate")}
            </Label>
            <Input
              id="v_depr"
              inputMode="decimal"
              value={form.annual_depreciation_rate}
              onChange={(e) =>
                setForm({
                  ...form,
                  annual_depreciation_rate: e.target.value,
                })
              }
              placeholder={t("assets:vehicle.placeholders.depreciationRate")}
            />
          </div>

          <div className="grid gap-2">
            <Label>{t("common:fields.ownership")}</Label>
            <div className="flex gap-4 text-sm">
              <label className="flex items-center gap-2">
                <input
                  type="radio"
                  name="v_ownership_type"
                  value="joint"
                  checked={form.ownership_type === "joint"}
                  onChange={() => setForm({ ...form, ownership_type: "joint" })}
                />
                {t("common:ownership.joint")}
              </label>
              <label className="flex items-center gap-2">
                <input
                  type="radio"
                  name="v_ownership_type"
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
                onChange={(e) =>
                  setForm({ ...form, sole_owner_user_id: e.target.value })
                }
              >
                {(members ?? []).map((m) => (
                  <option key={m.id} value={m.id}>
                    {preferredName(m)}
                    {user && m.id === user.id
                      ? t("common:ownership.youSuffix")
                      : ""}
                  </option>
                ))}
              </select>
            )}
          </div>

          <div className="grid gap-2">
            <Label htmlFor="v_description">
              {t("common:fields.description")}
            </Label>
            <Input
              id="v_description"
              value={form.description}
              onChange={(e) =>
                setForm({ ...form, description: e.target.value })
              }
            />
          </div>

          {mutation.error && (
            <p className="text-sm text-destructive">
              {errorMessage(mutation.error)}
            </p>
          )}

          <DialogFooter>
            <Button type="button" variant="outline" onClick={close}>
              {t("common:cancel")}
            </Button>
            <Button type="submit" disabled={mutation.isPending}>
              {mutation.isPending
                ? t("common:actions.creating")
                : t("common:actions.create")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
