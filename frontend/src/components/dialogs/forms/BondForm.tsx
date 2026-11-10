import { useTranslation } from "react-i18next";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import { NumericInput } from "@/components/common/NumericInput";
import { RiskProfileSelect } from "@/components/common/RiskProfileSelect";
import { OwnershipField } from "@/components/common/OwnershipField";
import { EntryTypeField } from "@/components/common/EntryTypeField";
import { RATE_DECIMALS, RATE_INTEGER_DIGITS, currencyDecimals } from "@/lib/format";
import type {
  BondType,
  CouponDisposition,
  CouponFrequency,
  EntryType,
  RiskProfile,
} from "@/api/types";
import type { FormMode } from "./mode";

export type BondFormState = {
  display_name: string;
  description: string;
  ownership_type: "sole" | "joint";
  entry_type: EntryType;
  sole_owner_user_id: string | null;
  risk_profile: RiskProfile | "";
  native_currency: string;
  bond_type: BondType;
  series_code: string;
  issuer: string;
  // Opening-position inputs: only Create asks for them (the held nominal is
  // derived from the Buy/Sell ledger afterwards, issue #27).
  face_value: string;
  placement_date: string;
  coupon_rate: string;
  coupon_frequency: CouponFrequency;
  coupon_disposition: CouponDisposition;
  maturity_date: string;
};

type Props = {
  mode: FormMode;
  form: BondFormState;
  setForm: (next: BondFormState) => void;
  soleOwnerID: string | null;
};

export function BondFormBody({ mode, form, setForm, soleOwnerID }: Props) {
  const { t } = useTranslation(["investments", "common"]);
  const opensWithFace = mode === "create" && form.bond_type === "govt_primary";

  // Currency is required for either bond type, but only the government-primary
  // create branch has a monetary field to sit beside — so the field is defined
  // once here and placed in both branches rather than duplicated (#572).
  const currencyField = (
    <div className="grid gap-2">
      <Label htmlFor="bond_currency">{t("common:fields.currency")}</Label>
      <Input
        id="bond_currency"
        required
        disabled={mode === "edit"}
        value={form.native_currency}
        onChange={(e) => setForm({ ...form, native_currency: e.target.value.toUpperCase() })}
        placeholder={t("investments:bond.placeholders.currency")}
        maxLength={3}
      />
    </div>
  );

  return (
    <>
      <div className="space-y-3">
        <div className="grid gap-2">
          <Label htmlFor="bond_display_name">{t("common:fields.displayName")}</Label>
          <Input
            id="bond_display_name"
            required
            value={form.display_name}
            onChange={(e) => setForm({ ...form, display_name: e.target.value })}
            placeholder={t("investments:bond.placeholders.displayName")}
          />
        </div>

        <div className="grid grid-cols-2 gap-3 [&>*]:content-end">
          <div className="grid gap-2">
            <Label htmlFor="bond_series_code">{t("investments:bond.fields.seriesCode")}</Label>
            <Input
              id="bond_series_code"
              value={form.series_code}
              onChange={(e) => setForm({ ...form, series_code: e.target.value })}
              placeholder={t("investments:bond.placeholders.seriesCode")}
            />
          </div>
          <div className="grid gap-2">
            <Label htmlFor="bond_issuer">{t("investments:bond.fields.issuer")}</Label>
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
          <Label htmlFor="bond_description">{t("common:fields.description")}</Label>
          <Input
            id="bond_description"
            value={form.description}
            onChange={(e) => setForm({ ...form, description: e.target.value })}
          />
        </div>
      </div>

      <div className="space-y-3 border-t pt-4">
        {/* Bond type takes a full-width row of its own: its longest option
            overran the ~170px half-row it used to share, and the dialog is
            `sm:max-w-sm` at every width, so there is no wider layout to fall
            back to (#572). */}
        <div className="grid gap-2">
          <Label htmlFor="bond_type">{t("investments:bond.fields.bondType")}</Label>
          <Select
            id="bond_type"
            value={form.bond_type}
            onChange={(e) => setForm({ ...form, bond_type: e.target.value as BondType })}
          >
            <option value="govt_primary">{t("investments:bond.bondType.govt_primary")}</option>
            <option value="secondary_market">
              {t("investments:bond.bondType.secondary_market")}
            </option>
          </Select>
        </div>

        {opensWithFace ? (
          <>
            {/* Currency sits beside the amount it denominates, at the same
                `1fr_120px` split the income dialog uses for amount+currency —
                three characters don't earn a full row. The placement date drops
                below the pair rather than sharing with the face value. */}
            <div className="grid grid-cols-[1fr_120px] gap-3 [&>*]:content-end">
              <div className="grid gap-2">
                <Label htmlFor="bond_face_value">{t("investments:bond.fields.faceValue")}</Label>
                <NumericInput
                  id="bond_face_value"
                  required
                  decimals={currencyDecimals(form.native_currency)}
                  value={form.face_value}
                  onChange={(v) => setForm({ ...form, face_value: v })}
                  placeholder={t("investments:bond.placeholders.faceValue")}
                />
              </div>
              {currencyField}
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
                onChange={(e) => setForm({ ...form, placement_date: e.target.value })}
              />
            </div>
          </>
        ) : (
          <>
            {mode === "create" && (
              <p className="text-sm text-muted-foreground">
                {t("investments:bond.secondaryBuyHint")}
              </p>
            )}
            {/* No monetary field to pair with here (a secondary-market position
                is opened by a Buy transaction, and Edit never asks for the face
                value), so currency keeps a row to itself. */}
            {currencyField}
          </>
        )}
      </div>

      <div className="space-y-3 border-t pt-4">
        <div className="grid grid-cols-2 gap-3 [&>*]:content-end">
          <div className="grid gap-2">
            <Label htmlFor="bond_coupon_rate">{t("investments:bond.fields.couponRate")}</Label>
            <NumericInput
              id="bond_coupon_rate"
              required
              decimals={RATE_DECIMALS}
              maxIntegerDigits={RATE_INTEGER_DIGITS}
              value={form.coupon_rate}
              onChange={(v) => setForm({ ...form, coupon_rate: v })}
              placeholder={t("investments:bond.placeholders.couponRate")}
            />
          </div>
          <div className="grid gap-2">
            <Label htmlFor="bond_coupon_frequency">
              {t("investments:bond.fields.couponFrequency")}
            </Label>
            <Select
              id="bond_coupon_frequency"
              value={form.coupon_frequency}
              onChange={(e) =>
                setForm({ ...form, coupon_frequency: e.target.value as CouponFrequency })
              }
            >
              <option value="monthly">{t("investments:bond.couponFrequency.monthly")}</option>
              <option value="quarterly">{t("investments:bond.couponFrequency.quarterly")}</option>
              <option value="semi_annual">
                {t("investments:bond.couponFrequency.semi_annual")}
              </option>
              <option value="annual">{t("investments:bond.couponFrequency.annual")}</option>
            </Select>
          </div>
        </div>

        <div className="grid gap-2">
          <Label htmlFor="bond_coupon_disposition">
            {t("investments:bond.fields.couponDisposition")}
          </Label>
          <Select
            id="bond_coupon_disposition"
            value={form.coupon_disposition}
            onChange={(e) =>
              setForm({ ...form, coupon_disposition: e.target.value as CouponDisposition })
            }
          >
            <option value="pays_out">{t("investments:bond.couponDisposition.pays_out")}</option>
            <option value="accrues">{t("investments:bond.couponDisposition.accrues")}</option>
          </Select>
          <p className="text-xs text-muted-foreground">
            {t("investments:bond.couponDisposition.hint")}
          </p>
        </div>

        <div className="grid gap-2">
          <Label htmlFor="bond_maturity">{t("investments:bond.fields.maturityDate")}</Label>
          <Input
            id="bond_maturity"
            required
            type="date"
            max="9999-12-31"
            value={form.maturity_date}
            onChange={(e) => setForm({ ...form, maturity_date: e.target.value })}
          />
        </div>
      </div>

      <div className="space-y-3 border-t pt-4">
        <OwnershipField
          idPrefix={`bond_${mode}`}
          value={form.ownership_type}
          onChange={(v) => setForm({ ...form, ownership_type: v })}
          soleOwnerID={soleOwnerID}
          onSoleOwnerChange={(v) => setForm({ ...form, sole_owner_user_id: v })}
        />

        <EntryTypeField
          idPrefix={`bond_${mode}`}
          group="investment"
          value={form.entry_type}
          onChange={(v) => setForm({ ...form, entry_type: v })}
        />
      </div>

      <RiskProfileSelect
        idPrefix={`bond_${mode}`}
        value={form.risk_profile}
        onChange={(v) => setForm({ ...form, risk_profile: v })}
      />
    </>
  );
}
