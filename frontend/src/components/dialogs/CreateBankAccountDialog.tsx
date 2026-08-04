import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useCreateBankAccount } from "@/hooks/useBankAccounts";
import { useSession } from "@/hooks/useSession";
import { PositionFormDialog } from "@/components/dialogs/PositionFormDialog";
import { OwnershipField } from "@/components/common/OwnershipField";
import { EntryTypeField } from "@/components/common/EntryTypeField";
import type { EntryType } from "@/api/types";
import { Select } from "@/components/ui/select";

const empty = {
  display_name: "",
  description: "",
  ownership_type: "joint" as "sole" | "joint",
  entry_type: "acquired" as EntryType,
  sole_owner_user_id: null as string | null,
  native_currency: "IDR",
  bank_name: "",
  account_number: "",
  account_type: "savings" as "savings" | "current" | "other",
};

export function CreateBankAccountDialog() {
  const { t } = useTranslation(["assets", "common"]);
  const [form, setForm] = useState(empty);
  const { data: user } = useSession();
  const mutation = useCreateBankAccount();

  const effectiveSoleOwnerID = form.sole_owner_user_id ?? user?.id ?? null;

  function submit(close: () => void) {
    if (!user) return;
    mutation.mutate(
      {
        display_name: form.display_name,
        description: form.description || null,
        ownership_type: form.ownership_type,
        entry_type: form.entry_type,
        sole_owner_user_id: form.ownership_type === "sole" ? effectiveSoleOwnerID : null,
        native_currency: form.native_currency,
        bank_name: form.bank_name,
        account_number: form.account_number,
        account_type: form.account_type,
      },
      { onSuccess: close },
    );
  }

  return (
    <PositionFormDialog
      trigger={
        <Button>
          <Plus className="mr-1 size-4" />
          {t("assets:bankAccount.createTrigger")}
        </Button>
      }
      title={t("assets:bankAccount.createTitle")}
      description={t("assets:bankAccount.createDescription")}
      submitLabel={t("common:actions.create")}
      pendingLabel={t("common:actions.creating")}
      isPending={mutation.isPending}
      error={mutation.error}
      onSubmit={submit}
      onClosed={() => {
        setForm(empty);
        mutation.reset();
      }}
    >
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
              setForm({
                ...form,
                account_type: e.target.value as typeof form.account_type,
              })
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
            value={form.native_currency}
            onChange={(e) =>
              setForm({
                ...form,
                native_currency: e.target.value.toUpperCase(),
              })
            }
            placeholder={t("assets:bankAccount.placeholders.currency")}
            maxLength={3}
          />
        </div>
      </div>

      <OwnershipField
        idPrefix="ba_create"
        value={form.ownership_type}
        onChange={(v) => setForm({ ...form, ownership_type: v })}
        soleOwnerID={effectiveSoleOwnerID}
        onSoleOwnerChange={(v) => setForm({ ...form, sole_owner_user_id: v })}
      />

      <EntryTypeField
        idPrefix="ba_create"
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
    </PositionFormDialog>
  );
}
