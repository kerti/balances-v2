import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useCreateGold, type GoldForm } from "@/hooks/useInvestments";
import { useSession } from "@/hooks/useSession";
import { PositionFormDialog } from "@/components/dialogs/PositionFormDialog";
import { GoldFormBody } from "@/components/dialogs/forms/GoldForm";
import type { RiskProfile, EntryType } from "@/api/types";

function emptyForm() {
  return {
    display_name: "",
    description: "",
    ownership_type: "sole" as "sole" | "joint",
    entry_type: "acquired" as EntryType,
    sole_owner_user_id: null as string | null,
    risk_profile: "" as RiskProfile | "",
    native_currency: "IDR",
    form: "bar" as GoldForm,
    purity: "0.9999",
  };
}

type Props = {
  // Called with the new position's id after a successful create, so the list
  // can land on its detail page (#843).
  onCreated?: (id: string) => void;
};

export function CreateGoldDialog({ onCreated }: Props) {
  const { t } = useTranslation(["investments", "common"]);
  const [form, setForm] = useState(emptyForm);
  const { data: user } = useSession();
  const mutation = useCreateGold();
  const [tagId, setTagId] = useState<string | null>(null);

  const effectiveSoleOwnerID = form.sole_owner_user_id ?? user?.id ?? null;

  function submit(close: () => void) {
    if (!user) return;
    if (!form.risk_profile) return;
    mutation.mutate(
      {
        display_name: form.display_name,
        description: form.description || null,
        ownership_type: form.ownership_type,
        entry_type: form.entry_type,
        tag_id: tagId,
        sole_owner_user_id: form.ownership_type === "sole" ? effectiveSoleOwnerID : null,
        risk_profile: form.risk_profile,
        native_currency: form.native_currency,
        form: form.form,
        purity: form.purity,
      },
      {
        onSuccess: (created) => {
          close();
          onCreated?.(created.investment.id);
        },
      },
    );
  }

  return (
    <PositionFormDialog
      trigger={
        <Button>
          <Plus className="mr-1 size-4" />
          {t("investments:gold.createTrigger")}
        </Button>
      }
      title={t("investments:gold.createTitle")}
      description={t("investments:gold.createDescription")}
      submitLabel={t("common:actions.create")}
      pendingLabel={t("common:actions.creating")}
      isPending={mutation.isPending}
      tag={{ value: tagId, onChange: setTagId }}
      error={mutation.error}
      onSubmit={submit}
      onClosed={() => {
        setForm(emptyForm());
        setTagId(null);
        mutation.reset();
      }}
    >
      <GoldFormBody
        mode="create"
        form={form}
        setForm={setForm}
        soleOwnerID={effectiveSoleOwnerID}
      />
    </PositionFormDialog>
  );
}
