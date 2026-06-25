import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useUpdateBond } from "@/hooks/useInvestments";
import { useHouseholdMembers } from "@/hooks/useHouseholdMembers";
import { preferredName } from "@/lib/names";
import { useSession } from "@/hooks/useSession";
import { errorMessage } from "@/lib/errorMessage";
import { RiskProfileSelect } from "@/components/RiskProfileSelect";
import type {
  Bond,
  BondListItem,
  BondType,
  CouponFrequency,
  CouponDisposition,
} from "@/api/types";

type Props = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  bond: Bond | BondListItem;
};

function toForm(bond: Bond | BondListItem) {
  const i = bond.investment;
  const d = bond.details;
  return {
    display_name: i.display_name,
    description: i.description ?? "",
    ownership_type: i.ownership_type,
    sole_owner_user_id: i.sole_owner_user_id,
    risk_profile: bond.investment.risk_profile,
    bond_type: d.bond_type,
    series_code: d.series_code ?? "",
    issuer: d.issuer,
    coupon_rate: d.coupon_rate,
    coupon_frequency: d.coupon_frequency,
    coupon_disposition: d.coupon_disposition,
    maturity_date: d.maturity_date ? d.maturity_date.slice(0, 10) : "",
  };
}

export function EditBondDialog({ open, onOpenChange, bond }: Props) {
  const { t } = useTranslation(["investments", "common"]);
  const [form, setForm] = useState(() => toForm(bond));
  const { data: user } = useSession();
  const { data: members } = useHouseholdMembers();
  const mutation = useUpdateBond(bond.investment.id);

  const effectiveSoleOwnerID = form.sole_owner_user_id ?? user?.id ?? null;

  function submit(e: React.FormEvent) {
    e.preventDefault();
    mutation.mutate(
      {
        display_name: form.display_name,
        description: form.description || null,
        ownership_type: form.ownership_type,
        sole_owner_user_id:
          form.ownership_type === "sole" ? effectiveSoleOwnerID : null,
        risk_profile: form.risk_profile,
        bond_type: form.bond_type,
        series_code: form.series_code.trim() || null,
        issuer: form.issuer,
        coupon_rate: form.coupon_rate,
        coupon_frequency: form.coupon_frequency,
        coupon_disposition: form.coupon_disposition,
        maturity_date: form.maturity_date,
      },
      { onSuccess: () => onOpenChange(false) },
    );
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{t("investments:bond.editTitle")}</DialogTitle>
          <DialogDescription>
            {t("investments:bond.editDescription")}
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={submit} className="space-y-4">
          <div className="space-y-3">
            <div className="grid gap-2">
              <Label htmlFor="edit_bond_display_name">
                {t("common:fields.displayName")}
              </Label>
              <Input
                id="edit_bond_display_name"
                required
                value={form.display_name}
                onChange={(e) =>
                  setForm({ ...form, display_name: e.target.value })
                }
              />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="grid gap-2">
                <Label htmlFor="edit_bond_series_code">
                  {t("investments:bond.fields.seriesCode")}
                </Label>
                <Input
                  id="edit_bond_series_code"
                  value={form.series_code}
                  onChange={(e) =>
                    setForm({ ...form, series_code: e.target.value })
                  }
                />
              </div>
              <div className="grid gap-2">
                <Label htmlFor="edit_bond_issuer">
                  {t("investments:bond.fields.issuer")}
                </Label>
                <Input
                  id="edit_bond_issuer"
                  required
                  value={form.issuer}
                  onChange={(e) => setForm({ ...form, issuer: e.target.value })}
                />
              </div>
            </div>
            <div className="grid gap-2">
              <Label htmlFor="edit_bond_description">
                {t("common:fields.description")}
              </Label>
              <Input
                id="edit_bond_description"
                value={form.description}
                onChange={(e) =>
                  setForm({ ...form, description: e.target.value })
                }
              />
            </div>
          </div>

          <div className="space-y-3 border-t pt-4">
            <div className="grid gap-2">
              <Label htmlFor="edit_bond_type">
                {t("investments:bond.fields.bondType")}
              </Label>
              <select
                id="edit_bond_type"
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
            {/* Outstanding nominal is derived from the Buy/Sell ledger (issue
                #27); edit it by adding/editing transactions, not here. */}
          </div>

          <div className="space-y-3 border-t pt-4">
            <div className="grid grid-cols-2 gap-3">
              <div className="grid gap-2">
                <Label htmlFor="edit_bond_coupon_rate">
                  {t("investments:bond.fields.couponRate")}
                </Label>
                <Input
                  id="edit_bond_coupon_rate"
                  required
                  inputMode="decimal"
                  value={form.coupon_rate}
                  onChange={(e) =>
                    setForm({ ...form, coupon_rate: e.target.value })
                  }
                />
              </div>
              <div className="grid gap-2">
                <Label htmlFor="edit_bond_coupon_frequency">
                  {t("investments:bond.fields.couponFrequency")}
                </Label>
                <select
                  id="edit_bond_coupon_frequency"
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
              <Label htmlFor="edit_bond_coupon_disposition">
                {t("investments:bond.fields.couponDisposition")}
              </Label>
              <select
                id="edit_bond_coupon_disposition"
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
              <Label htmlFor="edit_bond_maturity">
                {t("investments:bond.fields.maturityDate")}
              </Label>
              <Input
                id="edit_bond_maturity"
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
                    name="edit_bond_ownership_type"
                    value="joint"
                    checked={form.ownership_type === "joint"}
                    onChange={() =>
                      setForm({ ...form, ownership_type: "joint" })
                    }
                  />
                  {t("investments:ownership.joint")}
                </label>
                <label className="flex items-center gap-2">
                  <input
                    type="radio"
                    name="edit_bond_ownership_type"
                    value="sole"
                    checked={form.ownership_type === "sole"}
                    onChange={() =>
                      setForm({ ...form, ownership_type: "sole" })
                    }
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
            idPrefix="bond_edit"
            value={form.risk_profile}
            onChange={(v) => setForm({ ...form, risk_profile: v })}
          />

          {mutation.error && (
            <p className="text-sm text-destructive">
              {errorMessage(mutation.error)}
            </p>
          )}

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
            >
              {t("common:cancel")}
            </Button>
            <Button type="submit" disabled={mutation.isPending}>
              {mutation.isPending
                ? t("common:actions.saving")
                : t("common:actions.saveChanges")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
