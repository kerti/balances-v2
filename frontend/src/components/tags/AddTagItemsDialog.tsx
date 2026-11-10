import { useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { FormError, ValidatedForm } from "@/components/common/ValidatedForm";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { FieldError, useFieldError } from "@/components/ui/field-error";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import { useAssignTagsBulk, useTaggableItems, useTags } from "@/hooks/useTags";
import { groupItemsByType, itemKey } from "@/lib/tagItems";
import { PLAIN_TOAST_MS } from "@/lib/writeToast";
import type { TaggableItem } from "@/api/types";

type Props = {
  /** The tag the items go to; null = opened from Untagged, so the dialog asks. */
  tagId: string | null;
  tagName: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
};

function PickError() {
  const { message } = useFieldError("pick_items");
  return message ? <FieldError id="pick_items">{message}</FieldError> : null;
}

// The picker form (#853). Mounted only while the dialog is open, so the
// selection and search start empty each time without an effect.
function PickerForm({
  tagId,
  tagName,
  items,
  onDone,
}: {
  tagId: string | null;
  tagName: string;
  items: TaggableItem[];
  onDone: () => void;
}) {
  const { t } = useTranslation(["tags", "common"]);
  const { data: tags } = useTags();
  const mutation = useAssignTagsBulk();
  const [target, setTarget] = useState<string>(tagId ?? "");
  const [query, setQuery] = useState("");
  const [picked, setPicked] = useState<Set<string>>(new Set());

  const tagNameById = new Map((tags ?? []).map((tg) => [tg.id, tg.name]));
  // The tag the items land in: the row's own, or the one chosen from Untagged.
  const targetId = tagId ?? (target || null);
  const targetName = tagId ? tagName : (tagNameById.get(target) ?? "");

  // Active items only (a terminated one is not an "active item"); anything
  // already in the target tag has nothing to add.
  const candidates = items.filter(
    (it) => !it.terminated && (targetId === null || it.tag_id !== targetId),
  );
  const needle = query.trim().toLowerCase();
  const visible = needle
    ? candidates.filter((it) => it.name.toLowerCase().includes(needle))
    : candidates;
  const groups = groupItemsByType(visible);

  function toggle(key: string) {
    setPicked((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  }

  const chosen = candidates.filter((it) => picked.has(itemKey(it)));

  function validate() {
    const out: Record<string, string> = {};
    if (tagId === null && !target) out.target_tag = t("tags:items.picker.noTag");
    if (chosen.length === 0) out.pick_items = t("tags:items.picker.noneSelected");
    return out;
  }

  function submit() {
    const moved = chosen.filter((it) => it.tag_id !== null).length;
    mutation.mutate(
      {
        tag_id: targetId,
        items: chosen.map((it) => ({ group: it.group, position_id: it.id })),
      },
      {
        onSuccess: () => {
          const key = moved > 0 ? "tags:items.toast.addedMoved" : "tags:items.toast.added";
          toast.success(t(key, { count: chosen.length, moved, tag: targetName }), {
            duration: PLAIN_TOAST_MS,
          });
          onDone();
        },
      },
    );
  }

  return (
    <ValidatedForm
      onSubmit={submit}
      validate={validate}
      error={mutation.error}
      className="space-y-3"
    >
      {tagId === null && (
        <div className="grid gap-2">
          <Label htmlFor="target_tag">{t("tags:items.picker.targetLabel")}</Label>
          <Select
            id="target_tag"
            required
            data-testid="add-items-target"
            value={target}
            onChange={(e) => setTarget(e.target.value)}
          >
            <option value="">{t("tags:items.picker.targetPlaceholder")}</option>
            {(tags ?? []).map((tg) => (
              <option key={tg.id} value={tg.id}>
                {tg.name}
              </option>
            ))}
          </Select>
        </div>
      )}

      <Input
        type="search"
        aria-label={t("tags:items.picker.search")}
        placeholder={t("tags:items.picker.search")}
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        data-testid="add-items-search"
      />

      <div
        id="pick_items"
        tabIndex={-1}
        className="max-h-[50vh] space-y-3 overflow-y-auto outline-none"
        data-testid="add-items-list"
      >
        {groups.length === 0 && (
          <p className="text-sm text-muted-foreground" data-testid="add-items-empty">
            {t("tags:items.picker.empty")}
          </p>
        )}
        {groups.map((g) => (
          <section key={g.subtype} aria-label={t(`tags:itemType.${g.subtype}`)}>
            <h3 className="mb-1 text-xs font-medium uppercase tracking-wide text-muted-foreground">
              {t(`tags:itemType.${g.subtype}`)}
            </h3>
            <ul className="space-y-1">
              {g.items.map((it) => {
                const key = itemKey(it);
                const current = it.tag_id ? tagNameById.get(it.tag_id) : undefined;
                return (
                  <li key={key}>
                    <label className="flex min-h-11 cursor-pointer items-center gap-2 rounded-md border px-3">
                      <input
                        type="checkbox"
                        className="size-4 shrink-0 accent-primary"
                        checked={picked.has(key)}
                        onChange={() => toggle(key)}
                        data-testid={`add-items-option-${it.id}`}
                      />
                      <span className="min-w-0 flex-1">
                        <span className="block truncate">{it.name}</span>
                        {current && (
                          <span className="block text-xs text-muted-foreground">
                            {t("tags:items.picker.currently", { tag: current })}
                          </span>
                        )}
                      </span>
                    </label>
                  </li>
                );
              })}
            </ul>
          </section>
        ))}
      </div>
      <PickError />
      <FormError />
      <DialogFooter>
        <Button type="button" variant="outline" onClick={onDone}>
          {t("common:cancel")}
        </Button>
        <Button type="submit" disabled={mutation.isPending} data-testid="add-items-save">
          {t("tags:items.picker.save", { count: chosen.length })}
        </Button>
      </DialogFooter>
    </ValidatedForm>
  );
}

// "Add items…" on an expanded tag, "Add to a tag" on Untagged (#853): a
// searchable multi-select of active items grouped by type. An item that
// already carries another tag says so, and adding it moves it (ADR-0028: one
// tag per item). One save, one toast.
export function AddTagItemsDialog({ tagId, tagName, open, onOpenChange }: Props) {
  const { t } = useTranslation(["tags"]);
  const { data: items } = useTaggableItems();

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>
            {tagId
              ? t("tags:items.picker.title", { tag: tagName })
              : t("tags:items.picker.titleUntagged")}
          </DialogTitle>
          <DialogDescription>{t("tags:items.picker.description")}</DialogDescription>
        </DialogHeader>
        <PickerForm
          tagId={tagId}
          tagName={tagName}
          items={items ?? []}
          onDone={() => onOpenChange(false)}
        />
      </DialogContent>
    </Dialog>
  );
}
