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
import { useCreateProperty } from "@/hooks/useProperties";
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
  property_type: "house" as "house" | "apartment" | "land" | "commercial",
  address: "",
  acquisition_date: "",
  acquisition_cost: "",
  annual_appreciation_rate: "",
};

export function CreatePropertyDialog() {
  const { t } = useTranslation(["assets", "common"]);
  const [open, setOpen] = useState(false);
  const [form, setForm] = useState(empty);
  const { data: user } = useSession();
  const { data: members } = useHouseholdMembers();
  const mutation = useCreateProperty();

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
    <Dialog open={open} onOpenChange={(o) => (o ? setOpen(true) : close())}>
      <DialogTrigger asChild>
        <Button>
          <Plus className="mr-1 size-4" />
          {t("assets:property.createTrigger")}
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("assets:property.createTitle")}</DialogTitle>
          <DialogDescription>
            {t("assets:property.createDescription")}
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={submit} className="space-y-3">
          <div className="grid gap-2">
            <Label htmlFor="display_name">
              {t("common:fields.displayName")}
            </Label>
            <Input
              id="display_name"
              required
              value={form.display_name}
              onChange={(e) =>
                setForm({ ...form, display_name: e.target.value })
              }
              placeholder={t("assets:property.placeholders.displayName")}
            />
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div className="grid gap-2">
              <Label htmlFor="property_type">
                {t("assets:property.fields.type")}
              </Label>
              <select
                id="property_type"
                className="h-9 rounded-md border border-input bg-background px-3 text-sm"
                value={form.property_type}
                onChange={(e) =>
                  setForm({
                    ...form,
                    property_type: e.target.value as typeof form.property_type,
                  })
                }
              >
                <option value="house">
                  {t("assets:property.propertyTypes.house")}
                </option>
                <option value="apartment">
                  {t("assets:property.propertyTypes.apartment")}
                </option>
                <option value="land">
                  {t("assets:property.propertyTypes.land")}
                </option>
                <option value="commercial">
                  {t("assets:property.propertyTypes.commercial")}
                </option>
              </select>
            </div>
            <div className="grid gap-2">
              <Label htmlFor="native_currency">
                {t("common:fields.currency")}
              </Label>
              <Input
                id="native_currency"
                required
                value={form.native_currency}
                onChange={(e) =>
                  setForm({
                    ...form,
                    native_currency: e.target.value.toUpperCase(),
                  })
                }
                placeholder={t("assets:property.placeholders.currency")}
                maxLength={3}
              />
            </div>
          </div>

          <div className="grid gap-2">
            <Label htmlFor="address">
              {t("assets:property.fields.address")}
            </Label>
            <Input
              id="address"
              value={form.address}
              onChange={(e) => setForm({ ...form, address: e.target.value })}
            />
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div className="grid gap-2">
              <Label htmlFor="acquisition_date">
                {t("assets:property.fields.acquisitionDate")}
              </Label>
              <Input
                id="acquisition_date"
                type="date"
                max="9999-12-31"
                value={form.acquisition_date}
                onChange={(e) =>
                  setForm({ ...form, acquisition_date: e.target.value })
                }
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="acquisition_cost">
                {t("assets:property.fields.acquisitionCost")}
              </Label>
              <Input
                id="acquisition_cost"
                inputMode="decimal"
                value={form.acquisition_cost}
                onChange={(e) =>
                  setForm({ ...form, acquisition_cost: e.target.value })
                }
                placeholder={t("assets:property.placeholders.acquisitionCost")}
              />
            </div>
          </div>

          <div className="grid gap-2">
            <Label htmlFor="annual_appreciation_rate">
              {t("assets:property.fields.appreciationRate")}
            </Label>
            <Input
              id="annual_appreciation_rate"
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
                  name="ownership_type"
                  value="joint"
                  checked={form.ownership_type === "joint"}
                  onChange={() => setForm({ ...form, ownership_type: "joint" })}
                />
                {t("common:ownership.joint")}
              </label>
              <label className="flex items-center gap-2">
                <input
                  type="radio"
                  name="ownership_type"
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
            <Label htmlFor="description">
              {t("common:fields.description")}
            </Label>
            <Input
              id="description"
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
