import { useTranslation } from "react-i18next";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import { useTags } from "@/hooks/useTags";

// Single-select Tag dropdown for the position Create/Edit dialogs (ADR-0028).
// A Position carries at most one Tag, so this is a plain native <select>
// mirroring the Settings language/theme selects — value is the tag id, or ''
// for the "No tag" default. Native options can't render a colour swatch, so
// the dot lives on the TagBadge surfaces (list rows, report); here the name
// alone is enough to pick.
type Props = {
  value: string | null;
  onChange: (tagId: string | null) => void;
  id?: string;
  disabled?: boolean;
  // The dialogs stack the field's own "Tag" label above the select; the detail
  // page renders its own inline label beside it, so it opts out of this one.
  showLabel?: boolean;
};

export function TagSelect({ value, onChange, id = "tag", disabled, showLabel = true }: Props) {
  const { t } = useTranslation("tags");
  const { data: tags } = useTags();

  return (
    <div className="space-y-1">
      {showLabel && <Label htmlFor={id}>{t("field.label")}</Label>}
      <Select
        id={id}
        data-testid="tag-select"
        value={value ?? ""}
        onChange={(e) => onChange(e.target.value === "" ? null : e.target.value)}
        disabled={disabled}
      >
        <option value="">{t("field.none")}</option>
        {(tags ?? []).map((tag) => (
          <option key={tag.id} value={tag.id}>
            {tag.name}
          </option>
        ))}
      </Select>
    </div>
  );
}
