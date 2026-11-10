import { useTranslation } from "react-i18next";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { OwnershipField } from "@/components/common/OwnershipField";
import { EntryTypeField } from "@/components/common/EntryTypeField";
import type { EntryType } from "@/api/types";
import type { FormMode } from "./mode";

export type ReceivableFormState = {
  display_name: string;
  description: string;
  ownership_type: "sole" | "joint";
  entry_type: EntryType;
  sole_owner_user_id: string | null;
  native_currency: string;
  counterparty_name: string;
  due_date: string;
};

type Props = {
  mode: FormMode;
  form: ReceivableFormState;
  setForm: (next: ReceivableFormState) => void;
  soleOwnerID: string | null;
};

export function ReceivableFormBody({ mode, form, setForm, soleOwnerID }: Props) {
  const { t } = useTranslation(["receivables", "common"]);
  return (
    <>
      <div className="grid gap-2">
        <Label htmlFor="display_name">{t("common:fields.displayName")}</Label>
        <Input
          id="display_name"
          required
          value={form.display_name}
          onChange={(e) => setForm({ ...form, display_name: e.target.value })}
          placeholder={t("receivables:placeholders.displayName")}
        />
      </div>

      <div className="grid gap-2">
        <Label htmlFor="counterparty_name">{t("receivables:fields.counterparty")}</Label>
        <Input
          id="counterparty_name"
          required
          value={form.counterparty_name}
          onChange={(e) => setForm({ ...form, counterparty_name: e.target.value })}
          placeholder={t("receivables:placeholders.counterparty")}
        />
      </div>

      <div className="grid grid-cols-2 gap-3 [&>*]:content-end">
        <div className="grid gap-2">
          <Label htmlFor="native_currency">{t("common:fields.currency")}</Label>
          <Input
            id="native_currency"
            required
            disabled={mode === "edit"}
            value={form.native_currency}
            onChange={(e) => setForm({ ...form, native_currency: e.target.value.toUpperCase() })}
            placeholder={t("receivables:placeholders.currency")}
            maxLength={3}
          />
        </div>
        <div className="grid gap-2">
          <Label htmlFor="due_date">{t("receivables:fields.dueDate")}</Label>
          <Input
            id="due_date"
            type="date"
            max="9999-12-31"
            value={form.due_date}
            onChange={(e) => setForm({ ...form, due_date: e.target.value })}
          />
        </div>
      </div>

      <OwnershipField
        idPrefix={`receivable_${mode}`}
        value={form.ownership_type}
        onChange={(v) => setForm({ ...form, ownership_type: v })}
        soleOwnerID={soleOwnerID}
        onSoleOwnerChange={(v) => setForm({ ...form, sole_owner_user_id: v })}
      />

      <EntryTypeField
        idPrefix={`receivable_${mode}`}
        group="receivable"
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
