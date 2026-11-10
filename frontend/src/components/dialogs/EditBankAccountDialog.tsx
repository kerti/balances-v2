import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useUpdateBankAccount } from "@/hooks/useBankAccounts";
import { useSession } from "@/hooks/useSession";
import { PositionFormDialog } from "@/components/dialogs/PositionFormDialog";
import {
  BankAccountFormBody,
  type BankAccountFormState,
} from "@/components/dialogs/forms/BankAccountForm";
import type { BankAccount } from "@/api/types";

type Props = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  account: BankAccount;
};

function toForm(account: BankAccount): BankAccountFormState {
  return {
    display_name: account.asset.display_name,
    description: account.asset.description ?? "",
    ownership_type: account.asset.ownership_type,
    entry_type: account.asset.entry_type,
    sole_owner_user_id: account.asset.sole_owner_user_id,
    native_currency: account.asset.native_currency,
    bank_name: account.details.bank_name,
    account_number: account.details.account_number,
    account_type: account.details.account_type,
  };
}

// EditBankAccountDialog is controlled by the caller (no trigger). The parent
// passes open/onOpenChange and the account row to pre-fill from.
export function EditBankAccountDialog({ open, onOpenChange, account }: Props) {
  const { t } = useTranslation(["assets", "common"]);
  const mutation = useUpdateBankAccount(account.asset.id);
  const { data: user } = useSession();
  const [form, setForm] = useState(() => toForm(account));

  const effectiveSoleOwnerID = form.sole_owner_user_id ?? user?.id ?? null;

  function submit(close: () => void) {
    mutation.mutate(
      {
        display_name: form.display_name,
        description: form.description || null,
        ownership_type: form.ownership_type,
        entry_type: form.entry_type,
        sole_owner_user_id: form.ownership_type === "sole" ? effectiveSoleOwnerID : null,
        bank_name: form.bank_name,
        account_number: form.account_number,
        account_type: form.account_type,
      },
      { onSuccess: close },
    );
  }

  return (
    <PositionFormDialog
      open={open}
      onOpenChange={onOpenChange}
      title={t("assets:bankAccount.editTitle")}
      description={t("assets:bankAccount.editDescription")}
      submitLabel={t("common:actions.saveChanges")}
      pendingLabel={t("common:actions.saving")}
      isPending={mutation.isPending}
      error={mutation.error}
      onSubmit={submit}
    >
      <BankAccountFormBody
        mode="edit"
        form={form}
        setForm={setForm}
        soleOwnerID={effectiveSoleOwnerID}
      />
    </PositionFormDialog>
  );
}
