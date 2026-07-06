import { useTranslation } from "react-i18next";

type Props = {
  count: number;
  // Plural noun, e.g. "accounts" / "positions".
  nounPlural: string;
  checked: boolean;
  onChange: (checked: boolean) => void;
};

// Right-aligned "Show inactive {noun} (N)" checkbox shown above a list's table.
// Callers render it only when there are inactive rows to reveal, so the count
// is always ≥ 1 here.
export function ShowInactiveToggle({ count, nounPlural, checked, onChange }: Props) {
  const { t } = useTranslation("common");
  return (
    <div className="flex justify-end">
      <label className="flex items-center gap-2 text-sm text-muted-foreground">
        <input
          type="checkbox"
          className="h-4 w-4"
          checked={checked}
          onChange={(e) => onChange(e.target.checked)}
          data-testid="show-inactive"
        />
        {t("list.showInactive", { count, nounPlural })}
      </label>
    </div>
  );
}
