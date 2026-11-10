import { useTranslation } from "react-i18next";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import { OwnershipField } from "@/components/common/OwnershipField";
import { EntryTypeField } from "@/components/common/EntryTypeField";
import type { EntryType } from "@/api/types";
import type { FormMode } from "./mode";

export type BankAccountFormState = {
  display_name: string;
  description: string;
  ownership_type: "sole" | "joint";
  entry_type: EntryType;
  sole_owner_user_id: string | null;
  native_currency: string;
  bank_name: string;
  account_number: string;
  account_type: "savings" | "current" | "other";
};

type Props = {
  mode: FormMode;
  form: BankAccountFormState;
  setForm: (next: BankAccountFormState) => void;
  soleOwnerID: string | null;
};

// One field order for Create and Edit (#840). Currency is fixed once the
// account exists, so Edit shows it disabled in the same slot.
export function BankAccountFormBody({ mode, form, setForm, soleOwnerID }: Props) {
  const { t } = useTranslation(["assets", "common"]);
  return (
    <>
      <div className="grid gap-2">
        <Label htmlFor="display_name">{t("common:fields.displayName")}</Label>
        <Input
          id="display_name"
          required
          value={form.display_name}
          onChange={(e) => setForm({ ...form, display_name: e.target.value })}
          placeholder={t("assets:bankAccount.placeholders.displayName")}
        />
      </div>

      <div className="grid gap-2">
        <Label htmlFor="bank_name">{t("assets:bankAccount.fields.bankName")}</Label>
        <Input
          id="bank_name"
          required
          value={form.bank_name}
          onChange={(e) => setForm({ ...form, bank_name: e.target.value })}
          placeholder={t("assets:bankAccount.placeholders.bankName")}
        />
      </div>

      <div className="grid gap-2">
        <Label htmlFor="account_number">{t("assets:bankAccount.fields.accountNumber")}</Label>
        <Input
          id="account_number"
          required
          value={form.account_number}
          onChange={(e) => setForm({ ...form, account_number: e.target.value })}
        />
      </div>

      <div className="grid grid-cols-2 gap-3 [&>*]:content-end">
        <div className="grid gap-2">
          <Label htmlFor="account_type">{t("assets:bankAccount.fields.accountType")}</Label>
          <Select
            id="account_type"
            value={form.account_type}
            onChange={(e) =>
              setForm({ ...form, account_type: e.target.value as typeof form.account_type })
            }
          >
            <option value="savings">{t("assets:bankAccount.accountTypes.savings")}</option>
            <option value="current">{t("assets:bankAccount.accountTypes.current")}</option>
            <option value="other">{t("assets:bankAccount.accountTypes.other")}</option>
          </Select>
        </div>
        <div className="grid gap-2">
          <Label htmlFor="native_currency">{t("common:fields.currency")}</Label>
          <Input
            id="native_currency"
            required
            disabled={mode === "edit"}
            value={form.native_currency}
            onChange={(e) => setForm({ ...form, native_currency: e.target.value.toUpperCase() })}
            placeholder={t("assets:bankAccount.placeholders.currency")}
            maxLength={3}
          />
        </div>
      </div>

      <OwnershipField
        idPrefix={`ba_${mode}`}
        value={form.ownership_type}
        onChange={(v) => setForm({ ...form, ownership_type: v })}
        soleOwnerID={soleOwnerID}
        onSoleOwnerChange={(v) => setForm({ ...form, sole_owner_user_id: v })}
      />

      <EntryTypeField
        idPrefix={`ba_${mode}`}
        group="asset"
        value={form.entry_type}
        onChange={(v) => setForm({ ...form, entry_type: v })}
      />

      <div className="grid gap-2">
        <Label htmlFor="description">{t("common:fields.description")}</Label>
        <Input
          id="description"
          value={form.description}
          onChange={(e) => setForm({ ...form, description: e.target.value })}
        />
      </div>
    </>
  );
}
