import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Pencil, Trash2, Check, X } from "lucide-react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import { TagBadge } from "@/components/TagBadge";
import { useTags, useCreateTag, useUpdateTag, useDeleteTag } from "@/hooks/useTags";
import type { Tag } from "@/api/types";
import { TAG_SWATCHES, DEFAULT_TAG_COLOR } from "@/lib/tagColors";
import { errorMessage } from "@/lib/errorMessage";

// SwatchPicker renders the fixed palette as a row of selectable dots.
function SwatchPicker({
  value,
  onChange,
  disabled,
}: {
  value: string;
  onChange: (c: string) => void;
  disabled?: boolean;
}) {
  return (
    <div className="flex flex-wrap gap-1.5" role="radiogroup">
      {TAG_SWATCHES.map((c) => (
        <button
          key={c}
          type="button"
          role="radio"
          aria-checked={value === c}
          data-testid={`tag-swatch-${c}`}
          disabled={disabled}
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

// TagsCard is the Settings management surface for user-defined Tags (ADR-0028):
// create (name + swatch), inline rename/recolour, and delete. Mirrors the
// locale/theme cards in tone.
export function TagsCard() {
  const { t } = useTranslation("tags");
  const { data: tags } = useTags();
  const createTag = useCreateTag();

  const [name, setName] = useState("");
  const [color, setColor] = useState<string>(DEFAULT_TAG_COLOR);

  const onAdd = () => {
    const trimmed = name.trim();
    if (!trimmed) return;
    createTag.mutate(
      { name: trimmed, color },
      {
        onSuccess: () => {
          setName("");
          setColor(DEFAULT_TAG_COLOR);
        },
      },
    );
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{t("manage.title")}</CardTitle>
        <CardDescription>{t("manage.description")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {/* existing tags */}
        <div className="space-y-2" data-testid="tag-list">
          {(tags ?? []).length === 0 && (
            <p className="text-sm text-muted-foreground">{t("manage.empty")}</p>
          )}
          {(tags ?? []).map((tag) => (
            <TagRow key={tag.id} tag={tag} />
          ))}
        </div>

        {/* add form */}
        <div className="space-y-2 border-t pt-4">
          <Label htmlFor="new-tag-name">{t("manage.newLabel")}</Label>
          <div className="flex flex-wrap items-center gap-2">
            <Input
              id="new-tag-name"
              data-testid="new-tag-name"
              className="w-48"
              placeholder={t("manage.namePlaceholder")}
              value={name}
              maxLength={40}
              onChange={(e) => setName(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") onAdd();
              }}
              disabled={createTag.isPending}
            />
            <SwatchPicker value={color} onChange={setColor} disabled={createTag.isPending} />
            <Button
              type="button"
              data-testid="add-tag"
              onClick={onAdd}
              disabled={createTag.isPending || name.trim() === ""}
            >
              {t("manage.add")}
            </Button>
          </div>
          {createTag.isError && (
            <p className="text-sm text-destructive">{errorMessage(createTag.error)}</p>
          )}
        </div>
      </CardContent>
    </Card>
  );
}

function TagRow({ tag }: { tag: Tag }) {
  const { t } = useTranslation(["tags", "common"]);
  const updateTag = useUpdateTag(tag.id);
  const deleteTag = useDeleteTag();

  const [editing, setEditing] = useState(false);
  const [name, setName] = useState(tag.name);
  const [color, setColor] = useState(tag.color);
  const [confirmOpen, setConfirmOpen] = useState(false);

  const startEdit = () => {
    setName(tag.name);
    setColor(tag.color);
    setEditing(true);
  };

  const save = () => {
    const trimmed = name.trim();
    if (!trimmed) return;
    updateTag.mutate({ name: trimmed, color }, { onSuccess: () => setEditing(false) });
  };

  if (editing) {
    return (
      <div className="flex flex-wrap items-center gap-2">
        <Input
          className="w-48"
          data-testid="edit-tag-name"
          value={name}
          maxLength={40}
          onChange={(e) => setName(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") save();
            if (e.key === "Escape") setEditing(false);
          }}
          disabled={updateTag.isPending}
        />
        <SwatchPicker value={color} onChange={setColor} disabled={updateTag.isPending} />
        <Button
          type="button"
          size="icon"
          variant="ghost"
          data-testid="save-tag"
          onClick={save}
          disabled={updateTag.isPending || name.trim() === ""}
        >
          <Check className="size-4" />
        </Button>
        <Button
          type="button"
          size="icon"
          variant="ghost"
          onClick={() => setEditing(false)}
          disabled={updateTag.isPending}
        >
          <X className="size-4" />
        </Button>
        {updateTag.isError && (
          <p className="w-full text-sm text-destructive">{errorMessage(updateTag.error)}</p>
        )}
      </div>
    );
  }

  return (
    <div className="flex items-center justify-between gap-2">
      <TagBadge name={tag.name} color={tag.color} />
      <div className="flex items-center gap-1">
        <Button
          type="button"
          size="icon"
          variant="ghost"
          aria-label={t("manage.edit")}
          onClick={startEdit}
        >
          <Pencil className="size-4" />
        </Button>
        <Button
          type="button"
          size="icon"
          variant="ghost"
          aria-label={t("manage.delete")}
          data-testid="delete-tag"
          onClick={() => setConfirmOpen(true)}
        >
          <Trash2 className="size-4" />
        </Button>
      </div>
      <ConfirmDialog
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
        title={t("manage.deleteTitle", { name: tag.name })}
        description={t("manage.deleteDescription")}
        confirmLabel={t("common:delete")}
        destructive
        pending={deleteTag.isPending}
        onConfirm={() => deleteTag.mutate(tag.id, { onSuccess: () => setConfirmOpen(false) })}
      />
    </div>
  );
}
