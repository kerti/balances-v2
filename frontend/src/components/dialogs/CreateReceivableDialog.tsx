import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useCreateReceivable } from "@/hooks/useReceivables";
import { useSession } from "@/hooks/useSession";
import { PositionFormDialog } from "@/components/dialogs/PositionFormDialog";
import { ReceivableFormBody } from "@/components/dialogs/forms/ReceivableForm";
import type { EntryType } from "@/api/types";

const empty = {
  display_name: "",
  description: "",
  ownership_type: "joint" as "sole" | "joint",
  entry_type: "acquired" as EntryType,
  sole_owner_user_id: null as string | null,
  native_currency: "IDR",
  counterparty_name: "",
  due_date: "",
};

export function CreateReceivableDialog() {
  const { t } = useTranslation(["receivables", "common"]);
  const [form, setForm] = useState(empty);
  const { data: user } = useSession();
  const mutation = useCreateReceivable();

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
        counterparty_name: form.counterparty_name,
        due_date: form.due_date || null,
      },
      { onSuccess: close },
    );
  }

  return (
    <PositionFormDialog
      trigger={
        <Button>
          <Plus className="mr-1 size-4" />
          {t("receivables:createTrigger")}
        </Button>
      }
      title={t("receivables:createTitle")}
      description={t("receivables:createDescription")}
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
      <ReceivableFormBody
        mode="create"
        form={form}
        setForm={setForm}
        soleOwnerID={effectiveSoleOwnerID}
      />
    </PositionFormDialog>
  );
}
