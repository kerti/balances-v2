import { useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { TagSelect } from "@/components/TagSelect";
import { useAssignTag } from "@/hooks/useTags";
import { errorMessage } from "@/lib/errorMessage";
import type { TagGroup } from "@/api/types";

// DetailTagControl is the position-side assignment surface (ADR-0028, slice 2):
// a compact single-select Tag dropdown on each detail screen that writes
// straight through PUT /api/tags/assignments — no edit-dialog round trip.
// Local state drives the optimistic switch; the mutation persists it and
// invalidates the breakdown. Seeded once from the entity's tag_id; the parent
// passes key={positionId} on remount so a position switch re-seeds.
//
// Buttonless autosave (issue #54, ADR-0032): there is no Save button to confirm
// the write landed, so success and failure are reported via toast. On failure
// the optimistic value is rolled back so the dropdown keeps showing the truth.
type Props = {
  group: TagGroup;
  positionId: string;
  currentTagId: string | null;
};

export function DetailTagControl({ group, positionId, currentTagId }: Props) {
  const { t } = useTranslation("tags");
  const assign = useAssignTag();
  const [value, setValue] = useState<string | null>(currentTagId);

  const onChange = (tagId: string | null) => {
    const previous = value;
    setValue(tagId);
    assign.mutate(
      { group, position_id: positionId, tag_id: tagId },
      {
        onSuccess: () => toast.success(t("field.saved")),
        onError: (err) => {
          setValue(previous);
          toast.error(errorMessage(err));
        },
      },
    );
  };

  return (
    <div className="mt-2 max-w-[16rem]">
      <TagSelect
        value={value}
        onChange={onChange}
        id={`tag-${positionId}`}
        disabled={assign.isPending}
      />
    </div>
  );
}
