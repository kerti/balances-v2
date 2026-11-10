import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useCreateBankAccount } from "@/hooks/useBankAccounts";
import { useSession } from "@/hooks/useSession";
import { PositionFormDialog } from "@/components/dialogs/PositionFormDialog";
import {
  BankAccountFormBody,
  type BankAccountFormState,
} from "@/components/dialogs/forms/BankAccountForm";

const empty: BankAccountFormState = {
  display_name: "",
  description: "",
  ownership_type: "joint",
  entry_type: "acquired",
  sole_owner_user_id: null,
  native_currency: "IDR",
  bank_name: "",
  account_number: "",
  account_type: "savings",
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
      <BankAccountFormBody
        mode="create"
        form={form}
        setForm={setForm}
        soleOwnerID={effectiveSoleOwnerID}
      />
    </PositionFormDialog>
  );
}
