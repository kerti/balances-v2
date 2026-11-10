import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useUpdateReceivable } from "@/hooks/useReceivables";
import { useSession } from "@/hooks/useSession";
import { PositionFormDialog } from "@/components/dialogs/PositionFormDialog";
import {
  ReceivableFormBody,
  type ReceivableFormState,
} from "@/components/dialogs/forms/ReceivableForm";
import type { Receivable } from "@/api/types";

type Props = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  receivable: Receivable;
};

function toForm(r: Receivable): ReceivableFormState {
  return {
    display_name: r.display_name,
    description: r.description ?? "",
    ownership_type: r.ownership_type,
    entry_type: r.entry_type,
    sole_owner_user_id: r.sole_owner_user_id,
    native_currency: r.native_currency,
    counterparty_name: r.counterparty_name,
    due_date: r.due_date ? r.due_date.slice(0, 10) : "",
  };
}

export function EditReceivableDialog({ open, onOpenChange, receivable }: Props) {
  const { t } = useTranslation(["receivables", "common"]);
  const mutation = useUpdateReceivable(receivable.id);
  const { data: user } = useSession();
  const [form, setForm] = useState(() => toForm(receivable));

  const effectiveSoleOwnerID = form.sole_owner_user_id ?? user?.id ?? null;

  function submit(close: () => void) {
    mutation.mutate(
      {
        display_name: form.display_name,
        description: form.description || null,
        ownership_type: form.ownership_type,
        entry_type: form.entry_type,
        sole_owner_user_id: form.ownership_type === "sole" ? effectiveSoleOwnerID : null,
        counterparty_name: form.counterparty_name,
        due_date: form.due_date || null,
      },
      { onSuccess: close },
    );
  }

  return (
    <PositionFormDialog
      open={open}
      onOpenChange={onOpenChange}
      title={t("receivables:editTitle")}
      description={t("receivables:editDescription")}
      submitLabel={t("common:actions.saveChanges")}
      pendingLabel={t("common:actions.saving")}
      isPending={mutation.isPending}
      error={mutation.error}
      onSubmit={submit}
    >
      <ReceivableFormBody
        mode="edit"
        form={form}
        setForm={setForm}
        soleOwnerID={effectiveSoleOwnerID}
      />
    </PositionFormDialog>
  );
}
