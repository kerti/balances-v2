import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { PositionFormDialog } from "@/components/dialogs/PositionFormDialog";
import { useCreateTag, useUpdateTag } from "@/hooks/useTags";
import { TAG_SWATCHES, DEFAULT_TAG_COLOR } from "@/lib/tagColors";
import type { Tag } from "@/api/types";
import type { FormMode } from "@/components/dialogs/forms/mode";

type Props =
  | { mode: "create" }
  | { mode: "edit"; tag: Tag; open: boolean; onOpenChange: (open: boolean) => void };

type TagFormState = { name: string; color: string };

const empty: TagFormState = { name: "", color: DEFAULT_TAG_COLOR };

// SwatchPicker renders the fixed palette as a row of selectable dots.
function SwatchPicker({ value, onChange }: { value: string; onChange: (c: string) => void }) {
  return (
    <div className="flex flex-wrap gap-1.5" role="radiogroup">
      {TAG_SWATCHES.map((c) => (
        <button
          key={c}
          type="button"
          role="radio"
          aria-checked={value === c}
          data-testid={`tag-swatch-${c}`}
          onClick={() => onChange(c)}
          className={`size-6 rounded-full border-2 transition-transform ${
            value === c ? "scale-110 border-foreground" : "border-transparent hover:scale-105"
          }`}
          style={{ backgroundColor: c }}
        />
      ))}
    </div>
  );
}

function TagFormBody({
  mode,
  form,
  setForm,
}: {
  mode: FormMode;
  form: TagFormState;
  setForm: (next: TagFormState) => void;
}) {
  const { t } = useTranslation("tags");
  const nameTestId = mode === "create" ? "new-tag-name" : "edit-tag-name";
  return (
    <>
      <div className="grid gap-2">
        <Label htmlFor={nameTestId}>{t("manage.nameLabel")}</Label>
        <Input
          id={nameTestId}
          data-testid={nameTestId}
          placeholder={t("manage.namePlaceholder")}
          value={form.name}
          maxLength={40}
          onChange={(e) => setForm({ ...form, name: e.target.value })}
        />
      </div>
      <div className="grid gap-2">
        <Label>{t("manage.colorLabel")}</Label>
        <SwatchPicker value={form.color} onChange={(c) => setForm({ ...form, color: c })} />
      </div>
    </>
  );
}

// One dialog for creating and renaming/recolouring a Tag (ADR-0028). Delete
// stays a separate red confirm (ADR-0058: a tag delete has no undo).
export function TagDialog(props: Props) {
  const { t } = useTranslation(["tags", "common"]);
  const createTag = useCreateTag();
  const updateTag = useUpdateTag(props.mode === "edit" ? props.tag.id : "");
  const [form, setForm] = useState<TagFormState>(() =>
    props.mode === "edit" ? { name: props.tag.name, color: props.tag.color } : empty,
  );

  const valid = form.name.trim() !== "";
  const nameId = props.mode === "edit" ? "edit-tag-name" : "new-tag-name";
  const validate = () => (valid ? {} : { [nameId]: t("common:validation.required") });

  function submit(close: () => void) {
    const payload = { name: form.name.trim(), color: form.color };
    if (!valid) return;
    if (props.mode === "edit") {
      updateTag.mutate(payload, { onSuccess: close });
      return;
    }
    createTag.mutate(payload, { onSuccess: close });
  }

  const body = <TagFormBody mode={props.mode} form={form} setForm={setForm} />;

  if (props.mode === "edit") {
    return (
      <PositionFormDialog
        open={props.open}
        onOpenChange={props.onOpenChange}
        title={t("tags:manage.editTitle")}
        description={t("tags:manage.editDescription")}
        submitLabel={t("common:actions.saveChanges")}
        pendingLabel={t("common:actions.saving")}
        isPending={updateTag.isPending}
        validate={validate}
        error={updateTag.error}
        onSubmit={submit}
      >
        {body}
      </PositionFormDialog>
    );
  }

  return (
    <PositionFormDialog
      trigger={
        <Button size="sm" data-testid="new-tag">
          <Plus className="mr-1 size-4" />
          {t("tags:manage.newTag")}
        </Button>
      }
      title={t("tags:manage.createTitle")}
      description={t("tags:manage.createDescription")}
      submitLabel={t("tags:manage.add")}
      pendingLabel={t("common:actions.creating")}
      isPending={createTag.isPending}
      validate={validate}
      error={createTag.error}
      onSubmit={submit}
      onClosed={() => {
        setForm(empty);
        createTag.reset();
      }}
    >
      {body}
    </PositionFormDialog>
  );
}
