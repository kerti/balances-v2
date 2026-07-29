import { useTranslation } from "react-i18next";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import { useSession } from "@/hooks/useSession";
import { useHouseholdMembers } from "@/hooks/useHouseholdMembers";
import { preferredName } from "@/lib/names";

// The joint/sole ownership control, shared by all 22 position Create/Edit
// dialogs (#541). Every one of them had hand-inlined the same radio pair plus
// conditional sole-owner select; the copies differed only in the radio group's
// `name` and in whether they read the strings from the `common` or the
// `investments` namespace — and those two namespaces held byte-identical values
// for `ownership.joint` / `ownership.soleOwner` in both `en` and `id`, so the
// split bought nothing. This component standardises on `common`.
//
// The reason the duplication mattered rather than being merely untidy: the
// mobile tap floor could not be fixed in one place. Each copy rendered a bare
// `<input type="radio">` (~13px) inside a `flex items-center gap-2` label whose
// row height was ~20px — well under the 44px floor (INV-PRESENTATION-08 /
// ADR-0050). The `<label>` did already wrap its input, so tapping the *word*
// always worked; what was missing was a hit row big enough to aim at. Both
// options now sit in `max-md:min-h-11` rows that stretch to share the full
// width on phones, and collapse back to natural inline width from 768px up so
// the desktop form keeps its density.

type OwnershipType = "sole" | "joint";

type Props = {
  /** Disambiguating prefix for the radio group name, e.g. "td_create". */
  idPrefix: string;
  value: OwnershipType;
  onChange: (next: OwnershipType) => void;
  /**
   * The currently selected sole owner, already defaulted by the caller (which
   * needs the same resolved id for its submit payload, so it stays the owner of
   * that fallback rather than duplicating it here).
   */
  soleOwnerID: string | null;
  onSoleOwnerChange: (next: string) => void;
};

export function OwnershipField({
  idPrefix,
  value,
  onChange,
  soleOwnerID,
  onSoleOwnerChange,
}: Props) {
  const { t } = useTranslation("common");
  const { data: user } = useSession();
  const { data: members } = useHouseholdMembers();
  const name = `${idPrefix}_ownership_type`;

  return (
    <div className="grid gap-2">
      <Label>{t("fields.ownership")}</Label>
      <div
        data-testid="ownership-options"
        className="flex gap-2 text-sm max-md:w-full max-md:[&>*]:flex-1 md:gap-4"
      >
        {(["joint", "sole"] as const).map((option) => (
          <label
            key={option}
            className="flex items-center gap-2 max-md:min-h-11 max-md:rounded-lg max-md:border max-md:border-input max-md:px-3"
          >
            <input
              type="radio"
              name={name}
              value={option}
              checked={value === option}
              onChange={() => onChange(option)}
            />
            {t(option === "joint" ? "ownership.joint" : "ownership.soleOwner")}
          </label>
        ))}
      </div>
      {value === "sole" && (
        <Select
          aria-label={t("ownership.soleOwner")}
          value={soleOwnerID ?? ""}
          onChange={(e) => onSoleOwnerChange(e.target.value)}
        >
          {(members ?? []).map((m) => (
            <option key={m.id} value={m.id}>
              {preferredName(m)}
              {user && m.id === user.id ? t("ownership.youSuffix") : ""}
            </option>
          ))}
        </Select>
      )}
    </div>
  );
}
