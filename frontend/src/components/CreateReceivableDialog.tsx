import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useCreateReceivable } from "@/hooks/useReceivables";
import { useSession } from "@/hooks/useSession";
import { useHouseholdMembers } from "@/hooks/useHouseholdMembers";
import { preferredName } from "@/lib/names";
import { PositionFormDialog } from "@/components/PositionFormDialog";

const empty = {
  display_name: "",
  description: "",
  ownership_type: "joint" as "sole" | "joint",
  sole_owner_user_id: null as string | null,
  native_currency: "IDR",
  counterparty_name: "",
  due_date: "",
};

export function CreateReceivableDialog() {
  const { t } = useTranslation(["receivables", "common"]);
  const [form, setForm] = useState(empty);
  const { data: user } = useSession();
  const { data: members } = useHouseholdMembers();
  const mutation = useCreateReceivable();

  const effectiveSoleOwnerID = form.sole_owner_user_id ?? user?.id ?? null;

  function submit(close: () => void) {
    if (!user) return;
    mutation.mutate(
      {
        display_name: form.display_name,
        description: form.description || null,
        ownership_type: form.ownership_type,
        sole_owner_user_id:
          form.ownership_type === "sole" ? effectiveSoleOwnerID : null,
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
        <Label htmlFor="counterparty_name">
          {t("receivables:fields.counterparty")}
        </Label>
        <Input
          id="counterparty_name"
          required
          value={form.counterparty_name}
          onChange={(e) =>
            setForm({ ...form, counterparty_name: e.target.value })
          }
          placeholder={t("receivables:placeholders.counterparty")}
        />
      </div>

      <div className="grid grid-cols-2 gap-3">
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

      <div className="grid gap-2">
        <Label>{t("common:fields.ownership")}</Label>
        <div className="flex gap-4 text-sm">
          <label className="flex items-center gap-2">
            <input
              type="radio"
              name="ownership_type"
              value="joint"
              checked={form.ownership_type === "joint"}
              onChange={() => setForm({ ...form, ownership_type: "joint" })}
            />
            {t("common:ownership.joint")}
          </label>
          <label className="flex items-center gap-2">
            <input
              type="radio"
              name="ownership_type"
              value="sole"
              checked={form.ownership_type === "sole"}
              onChange={() => setForm({ ...form, ownership_type: "sole" })}
            />
            {t("common:ownership.soleOwner")}
          </label>
        </div>
        {form.ownership_type === "sole" && (
          <select
            aria-label={t("common:ownership.soleOwner")}
            className="h-9 rounded-md border border-input bg-background px-3 text-sm"
            value={effectiveSoleOwnerID ?? ""}
            onChange={(e) =>
              setForm({ ...form, sole_owner_user_id: e.target.value })
            }
          >
            {(members ?? []).map((m) => (
              <option key={m.id} value={m.id}>
                {preferredName(m)}
                {user && m.id === user.id
                  ? t("common:ownership.youSuffix")
                  : ""}
              </option>
            ))}
          </select>
        )}
      </div>

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
