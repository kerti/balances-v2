import { useState } from "react";
import { Link } from "react-router";
import { useTranslation } from "react-i18next";
import { Plus, X } from "lucide-react";
import { useQueryClient } from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
import { AddTagItemsDialog } from "@/components/tags/AddTagItemsDialog";
import { useAssignTagsBulk, useTaggableItems } from "@/hooks/useTags";
import { formatCurrency } from "@/lib/format";
import { itemDetailPath, itemKey } from "@/lib/tagItems";
import { toastUndoable, toastWriteError } from "@/lib/writeToast";
import type { TaggableItem } from "@/api/types";

type Props = {
  /** null = the Untagged bucket. */
  tagId: string | null;
  tagName: string;
  /** The breakdown section this row sits in; only that currency's items list here. */
  currency: string;
};

// The items under an expanded breakdown row (#853): name, type, current value
// in this section's currency (no FX, same as the row's totals), a link to the
// detail page and a remove x. One layout for desktop and phones: each item is a
// stacked card with 44px targets. Removing is a single-item write, so it takes
// the Undo tier of ADR-0058 (no confirm: re-adding is trivial); Untagged has no
// remove, its items carry no tag to remove.
export function TagItemsPanel({ tagId, tagName, currency }: Props) {
  const { t } = useTranslation(["tags", "common"]);
  const qc = useQueryClient();
  const { data: items } = useTaggableItems();
  const assign = useAssignTagsBulk();
  const [addOpen, setAddOpen] = useState(false);

  const mine = (items ?? []).filter((it) => it.tag_id === tagId && it.currency === currency);

  function remove(item: TaggableItem) {
    const ref = { group: item.group, position_id: item.id };
    assign.mutate(
      { tag_id: null, items: [ref] },
      {
        onSuccess: () =>
          toastUndoable({
            message: t("tags:items.toast.removed", { name: item.name, tag: tagName }),
            undoPath: "/api/tags/assignments/bulk",
            undoMethod: "PUT",
            undoBody: { tag_id: tagId, items: [ref] },
            testId: "toast-removed",
            qc,
            onUndone: () => qc.invalidateQueries(),
          }),
        onError: toastWriteError,
      },
    );
  }

  return (
    <div className="space-y-2" data-testid={`tag-items-${tagId ?? "untagged"}-${currency}`}>
      {mine.length === 0 && (
        <p className="text-sm text-muted-foreground" data-testid="tag-items-empty">
          {t(tagId ? "tags:items.empty" : "tags:items.emptyUntagged")}
        </p>
      )}
      <ul className="space-y-2">
        {mine.map((item) => {
          const path = itemDetailPath(item);
          return (
            <li
              key={itemKey(item)}
              className="flex items-center gap-2 rounded-lg border p-2 pl-3"
              data-testid="tag-item-row"
            >
              <div className="min-w-0 flex-1">
                {path ? (
                  <Link
                    to={path}
                    className="flex min-h-11 items-center truncate font-medium underline-offset-2 hover:underline"
                    data-testid="tag-item-link"
                  >
                    {item.name}
                  </Link>
                ) : (
                  <span className="block truncate font-medium">{item.name}</span>
                )}
                <span className="block text-xs text-muted-foreground">
                  {t(`tags:itemType.${item.subtype}`)}
                </span>
              </div>
              <span className="shrink-0 text-right text-sm" data-testid="tag-item-value">
                {item.amount === null
                  ? "—"
                  : `${item.group === "liability" ? "−" : ""}${formatCurrency(item.amount, currency)}`}
              </span>
              {tagId !== null && (
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  className="size-11 shrink-0"
                  aria-label={t("tags:items.remove", { name: item.name })}
                  disabled={assign.isPending}
                  onClick={() => remove(item)}
                  data-testid="tag-item-remove"
                >
                  <X className="size-4" />
                </Button>
              )}
            </li>
          );
        })}
      </ul>
      <Button
        type="button"
        variant="outline"
        className="w-full sm:w-auto"
        onClick={() => setAddOpen(true)}
        data-testid={tagId ? "tag-items-add" : "tag-items-add-untagged"}
      >
        <Plus className="mr-1 size-4" />
        {t(tagId ? "tags:items.add" : "tags:items.addUntagged")}
      </Button>
      {addOpen && (
        <AddTagItemsDialog
          tagId={tagId}
          tagName={tagName}
          open={addOpen}
          onOpenChange={setAddOpen}
        />
      )}
    </div>
  );
}
