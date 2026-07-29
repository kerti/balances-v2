import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useUpdateReceivable } from "@/hooks/useReceivables";
import { useSession } from "@/hooks/useSession";
import { PositionFormDialog } from "@/components/dialogs/PositionFormDialog";
import { OwnershipField } from "@/components/common/OwnershipField";
import type { Receivable } from "@/api/types";

type Props = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  receivable: Receivable;
};

function toForm(r: Receivable) {
  return {
    display_name: r.display_name,
    description: r.description ?? "",
    ownership_type: r.ownership_type,
    sole_owner_user_id: r.sole_owner_user_id,
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
      <div className="grid gap-2">
        <Label htmlFor="edit_r_display_name">{t("common:fields.displayName")}</Label>
        <Input
          id="edit_r_display_name"
          required
          value={form.display_name}
          onChange={(e) => setForm({ ...form, display_name: e.target.value })}
        />
      </div>

      <div className="grid gap-2">
        <Label htmlFor="edit_r_counterparty">{t("receivables:fields.counterparty")}</Label>
        <Input
          id="edit_r_counterparty"
          required
          value={form.counterparty_name}
          onChange={(e) => setForm({ ...form, counterparty_name: e.target.value })}
        />
      </div>

      <div className="grid gap-2">
        <Label htmlFor="edit_r_due_date">{t("receivables:fields.dueDate")}</Label>
        <Input
          id="edit_r_due_date"
          type="date"
          max="9999-12-31"
          value={form.due_date}
          onChange={(e) => setForm({ ...form, due_date: e.target.value })}
        />
      </div>

      <OwnershipField
        idPrefix="receivable_edit"
        value={form.ownership_type}
        onChange={(v) => setForm({ ...form, ownership_type: v })}
        soleOwnerID={effectiveSoleOwnerID}
        onSoleOwnerChange={(v) => setForm({ ...form, sole_owner_user_id: v })}
      />

      <div className="grid gap-2">
        <Label htmlFor="edit_r_description">{t("common:fields.description")}</Label>
        <Input
          id="edit_r_description"
          value={form.description}
          onChange={(e) => setForm({ ...form, description: e.target.value })}
        />
      </div>
    </PositionFormDialog>
  );
}
