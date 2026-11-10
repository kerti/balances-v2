import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Plus } from "lucide-react";
import { AddTagItemsDialog } from "@/components/tags/AddTagItemsDialog";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { ConfirmDialog } from "@/components/dialogs/ConfirmDialog";
import { TagDialog } from "@/components/dialogs/TagDialog";
import { RowActionsMenu } from "@/components/positionList/RowActionsMenu";
import { TagBadge } from "@/components/common/TagBadge";
import { useTags, useDeleteTag } from "@/hooks/useTags";
import type { Tag } from "@/api/types";

// TagsCard is the Settings management surface for user-defined Tags (ADR-0028).
// Create and rename/recolour both happen in `TagDialog` (#840); each row's ⋯
// menu offers Edit and Delete (Delete keeps its red confirm — a tag delete has
// no undo).
export function TagsCard() {
  const { t } = useTranslation("tags");
  const { data: tags } = useTags();

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{t("manage.title")}</CardTitle>
        <CardDescription>{t("manage.description")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="flex justify-end">
          <TagDialog mode="create" />
        </div>
        <div className="space-y-2" data-testid="tag-list">
          {(tags ?? []).length === 0 && (
            <p className="text-sm text-muted-foreground">{t("manage.empty")}</p>
          )}
          {(tags ?? []).map((tag) => (
            <TagRow key={tag.id} tag={tag} />
          ))}
        </div>
      </CardContent>
    </Card>
  );
}

function TagRow({ tag }: { tag: Tag }) {
  const { t } = useTranslation(["tags", "common"]);
  const deleteTag = useDeleteTag();
  const [editOpen, setEditOpen] = useState(false);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [addOpen, setAddOpen] = useState(false);

  return (
    <div className="flex items-center justify-between gap-2">
      <TagBadge name={tag.name} color={tag.color} />
      <RowActionsMenu
        label={t("manage.rowActions")}
        triggerClassName="max-md:size-11"
        onEdit={() => setEditOpen(true)}
        onDelete={() => setConfirmOpen(true)}
        extraAction={{
          label: t("items.add"),
          icon: <Plus />,
          onSelect: () => setAddOpen(true),
        }}
      />
      {addOpen && (
        <AddTagItemsDialog
          tagId={tag.id}
          tagName={tag.name}
          open={addOpen}
          onOpenChange={setAddOpen}
        />
      )}
      {editOpen && <TagDialog mode="edit" tag={tag} open={editOpen} onOpenChange={setEditOpen} />}
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
