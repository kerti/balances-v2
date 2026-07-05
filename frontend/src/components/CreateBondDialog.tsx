import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useCreateBond } from "@/hooks/useInvestments";
import { useSession } from "@/hooks/useSession";
import { useHouseholdMembers } from "@/hooks/useHouseholdMembers";
import { preferredName } from "@/lib/names";
import { RiskProfileSelect } from "@/components/RiskProfileSelect";
import { PositionFormDialog } from "@/components/PositionFormDialog";
import type { RiskProfile } from "@/api/types";
import type { BondType, CouponFrequency, CouponDisposition } from "@/api/types";

function emptyForm() {
  return {
    display_name: "",
    description: "",
    ownership_type: "joint" as "sole" | "joint",
    sole_owner_user_id: null as string | null,
    risk_profile: "" as RiskProfile | "",
    native_currency: "IDR",
    bond_type: "govt_primary" as BondType,
    series_code: "",
    issuer: "",
    face_value: "",
    placement_date: "",
    coupon_rate: "",
    coupon_frequency: "monthly" as CouponFrequency,
    coupon_disposition: "pays_out" as CouponDisposition,
    maturity_date: "",
  };
}

export function CreateBondDialog() {
  const { t } = useTranslation(["investments", "common"]);
  const [form, setForm] = useState(emptyForm);
  const { data: user } = useSession();
  const { data: members } = useHouseholdMembers();
  const mutation = useCreateBond();

  const effectiveSoleOwnerID = form.sole_owner_user_id ?? user?.id ?? null;

  function submit(close: () => void) {
    if (!user) return;
    if (!form.risk_profile) return;
    mutation.mutate(
      {
        display_name: form.display_name,
        description: form.description || null,
        ownership_type: form.ownership_type,
        sole_owner_user_id:
          form.ownership_type === "sole" ? effectiveSoleOwnerID : null,
        risk_profile: form.risk_profile,
        native_currency: form.native_currency,
        bond_type: form.bond_type,
        series_code: form.series_code.trim() || null,
        issuer: form.issuer,
        face_value: form.face_value,
        placement_date: form.placement_date,
        coupon_rate: form.coupon_rate,
        coupon_frequency: form.coupon_frequency,
        coupon_disposition: form.coupon_disposition,
        maturity_date: form.maturity_date,
      },
      { onSuccess: close },
    );
  }

  return (
    <PositionFormDialog
      trigger={
        <Button>
          <Plus className="mr-1 size-4" />
          {t("investments:bond.createTrigger")}
        </Button>
      }
      contentClassName="max-h-[90vh] overflow-y-auto"
      formClassName="space-y-4"
      title={t("investments:bond.createTitle")}
      description={t("investments:bond.createDescription")}
      submitLabel={t("common:actions.create")}
      pendingLabel={t("common:actions.creating")}
      isPending={mutation.isPending}
      error={mutation.error}
      onSubmit={submit}
      onClosed={() => {
        setForm(emptyForm());
        mutation.reset();
      }}
    >
      <div className="space-y-3">
        <div className="grid gap-2">
          <Label htmlFor="bond_display_name">
            {t("common:fields.displayName")}
          </Label>
          <Input
            id="bond_display_name"
            required
            value={form.display_name}
            onChange={(e) => setForm({ ...form, display_name: e.target.value })}
            placeholder={t("investments:bond.placeholders.displayName")}
          />
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div className="grid gap-2">
            <Label htmlFor="bond_series_code">
              {t("investments:bond.fields.seriesCode")}
            </Label>
            <Input
              id="bond_series_code"
              value={form.series_code}
              onChange={(e) =>
                setForm({ ...form, series_code: e.target.value })
              }
              placeholder={t("investments:bond.placeholders.seriesCode")}
            />
          </div>
          <div className="grid gap-2">
            <Label htmlFor="bond_issuer">
              {t("investments:bond.fields.issuer")}
            </Label>
            <Input
              id="bond_issuer"
              required
              value={form.issuer}
              onChange={(e) => setForm({ ...form, issuer: e.target.value })}
              placeholder={t("investments:bond.placeholders.issuer")}
            />
          </div>
        </div>

        <div className="grid gap-2">
          <Label htmlFor="bond_description">
            {t("common:fields.description")}
          </Label>
          <Input
            id="bond_description"
            value={form.description}
            onChange={(e) => setForm({ ...form, description: e.target.value })}
          />
        </div>
      </div>

      <div className="space-y-3 border-t pt-4">
        <div className="grid grid-cols-2 gap-3">
          <div className="grid gap-2">
            <Label htmlFor="bond_type">
              {t("investments:bond.fields.bondType")}
            </Label>
            <select
              id="bond_type"
              className="h-9 rounded-md border border-input bg-background px-3 text-sm"
              value={form.bond_type}
              onChange={(e) =>
                setForm({ ...form, bond_type: e.target.value as BondType })
              }
            >
              <option value="govt_primary">
                {t("investments:bond.bondType.govt_primary")}
              </option>
              <option value="secondary_market">
                {t("investments:bond.bondType.secondary_market")}
              </option>
            </select>
          </div>
          <div className="grid gap-2">
            <Label htmlFor="bond_currency">{t("common:fields.currency")}</Label>
            <Input
              id="bond_currency"
              required
              value={form.native_currency}
              onChange={(e) =>
                setForm({
                  ...form,
                  native_currency: e.target.value.toUpperCase(),
                })
              }
              placeholder={t("investments:bond.placeholders.currency")}
              maxLength={3}
            />
          </div>
        </div>

        {form.bond_type === "govt_primary" ? (
          <div className="grid grid-cols-2 gap-3">
            <div className="grid gap-2">
              <Label htmlFor="bond_face_value">
                {t("investments:bond.fields.faceValue")}
              </Label>
              <Input
                id="bond_face_value"
                required
                inputMode="decimal"
                value={form.face_value}
                onChange={(e) =>
                  setForm({ ...form, face_value: e.target.value })
                }
                placeholder={t("investments:bond.placeholders.faceValue")}
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="bond_placement_date">
                {t("investments:bond.fields.placementDate")}
              </Label>
              <Input
                id="bond_placement_date"
                required
                type="date"
                max="9999-12-31"
                value={form.placement_date}
                onChange={(e) =>
                  setForm({ ...form, placement_date: e.target.value })
                }
              />
            </div>
          </div>
        ) : (
          <p className="text-sm text-muted-foreground">
            {t("investments:bond.secondaryBuyHint")}
          </p>
        )}
      </div>

      <div className="space-y-3 border-t pt-4">
        <div className="grid grid-cols-2 gap-3">
          <div className="grid gap-2">
            <Label htmlFor="bond_coupon_rate">
              {t("investments:bond.fields.couponRate")}
            </Label>
            <Input
              id="bond_coupon_rate"
              required
              inputMode="decimal"
              value={form.coupon_rate}
              onChange={(e) =>
                setForm({ ...form, coupon_rate: e.target.value })
              }
              placeholder={t("investments:bond.placeholders.couponRate")}
            />
          </div>
          <div className="grid gap-2">
            <Label htmlFor="bond_coupon_frequency">
              {t("investments:bond.fields.couponFrequency")}
            </Label>
            <select
              id="bond_coupon_frequency"
              className="h-9 rounded-md border border-input bg-background px-3 text-sm"
              value={form.coupon_frequency}
              onChange={(e) =>
                setForm({
                  ...form,
                  coupon_frequency: e.target.value as CouponFrequency,
                })
              }
            >
              <option value="monthly">
                {t("investments:bond.couponFrequency.monthly")}
              </option>
              <option value="quarterly">
                {t("investments:bond.couponFrequency.quarterly")}
              </option>
              <option value="semi_annual">
                {t("investments:bond.couponFrequency.semi_annual")}
              </option>
              <option value="annual">
                {t("investments:bond.couponFrequency.annual")}
              </option>
            </select>
          </div>
        </div>

        <div className="grid gap-2">
          <Label htmlFor="bond_coupon_disposition">
            {t("investments:bond.fields.couponDisposition")}
          </Label>
          <select
            id="bond_coupon_disposition"
            className="h-9 rounded-md border border-input bg-background px-3 text-sm"
            value={form.coupon_disposition}
            onChange={(e) =>
              setForm({
                ...form,
                coupon_disposition: e.target.value as CouponDisposition,
              })
            }
          >
            <option value="pays_out">
              {t("investments:bond.couponDisposition.pays_out")}
            </option>
            <option value="accrues">
              {t("investments:bond.couponDisposition.accrues")}
            </option>
          </select>
          <p className="text-xs text-muted-foreground">
            {t("investments:bond.couponDisposition.hint")}
          </p>
        </div>

        <div className="grid gap-2">
          <Label htmlFor="bond_maturity">
            {t("investments:bond.fields.maturityDate")}
          </Label>
          <Input
            id="bond_maturity"
            required
            type="date"
            max="9999-12-31"
            value={form.maturity_date}
            onChange={(e) =>
              setForm({ ...form, maturity_date: e.target.value })
            }
          />
        </div>
      </div>

      <div className="space-y-3 border-t pt-4">
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
              {t("investments:ownership.joint")}
            </label>
            <label className="flex items-center gap-2">
              <input
                type="radio"
                name="ownership_type"
                value="sole"
                checked={form.ownership_type === "sole"}
                onChange={() => setForm({ ...form, ownership_type: "sole" })}
              />
              {t("investments:ownership.soleOwner")}
            </label>
          </div>
          {form.ownership_type === "sole" && (
            <select
              aria-label={t("investments:ownership.soleOwnerAria")}
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
      </div>

      <RiskProfileSelect
        idPrefix="bond_create"
        value={form.risk_profile}
        onChange={(v) => setForm({ ...form, risk_profile: v })}
      />
    </PositionFormDialog>
  );
}
