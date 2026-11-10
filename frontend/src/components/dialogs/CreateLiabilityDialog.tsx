import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useCreateLiability } from "@/hooks/useLiabilities";
import { useSession } from "@/hooks/useSession";
import { PositionFormDialog } from "@/components/dialogs/PositionFormDialog";
import { LiabilityFormBody } from "@/components/dialogs/forms/LiabilityForm";
import type { EntryType } from "@/api/types";

type Props = {
  // When the dialog opens from inside an inner-tab (Personal / Institutional),
  // the subtype is fixed by the tab — we pre-fill and hide the selector.
  defaultSubtype?: "personal" | "institutional";
};

function emptyForm(defaultSubtype: "personal" | "institutional") {
  return {
    display_name: "",
    description: "",
    subtype: defaultSubtype,
    ownership_type: "joint" as "sole" | "joint",
    entry_type: "acquired" as EntryType,
    sole_owner_user_id: null as string | null,
    native_currency: "IDR",
    counterparty_name: "",
    principal: "",
    interest_rate: "",
    term_months: "",
    start_date: "",
    maturity_date: "",
  };
}

export function CreateLiabilityDialog({ defaultSubtype = "personal" }: Props) {
  const { t } = useTranslation(["liabilities", "common"]);
  const [form, setForm] = useState(emptyForm(defaultSubtype));
  const { data: user } = useSession();
  const mutation = useCreateLiability();

  const effectiveSoleOwnerID = form.sole_owner_user_id ?? user?.id ?? null;

  function submit(close: () => void) {
    if (!user) return;
    mutation.mutate(
      {
        display_name: form.display_name,
        description: form.description || null,
        subtype: form.subtype,
        ownership_type: form.ownership_type,
        entry_type: form.entry_type,
        sole_owner_user_id: form.ownership_type === "sole" ? effectiveSoleOwnerID : null,
        native_currency: form.native_currency,
        counterparty_name: form.counterparty_name,
        principal: form.principal || null,
        interest_rate: form.interest_rate || null,
        term_months: form.term_months ? Number(form.term_months) : null,
        start_date: form.start_date || null,
        maturity_date: form.maturity_date || null,
      },
      { onSuccess: close },
    );
  }

  return (
    <PositionFormDialog
      trigger={
        <Button>
          <Plus className="mr-1 size-4" />
          {t("liabilities:createTrigger")}
        </Button>
      }
      title={t("liabilities:createTitle")}
      description={t("liabilities:createDescription")}
      submitLabel={t("common:actions.create")}
      pendingLabel={t("common:actions.creating")}
      isPending={mutation.isPending}
      error={mutation.error}
      onSubmit={submit}
      onClosed={() => {
        setForm(emptyForm(defaultSubtype));
        mutation.reset();
      }}
    >
      <LiabilityFormBody
        mode="create"
        form={form}
        setForm={setForm}
        soleOwnerID={effectiveSoleOwnerID}
      />
    </PositionFormDialog>
  );
}
