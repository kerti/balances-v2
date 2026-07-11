import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { GOLD_PURITY_PRESETS, goldPurityPresetKarat } from "@/lib/gold";

// Gold purity picker: a karat dropdown (24K/22K/.../10K) plus Custom, instead
// of forcing the user to type a decimal fraction. Picking a karat writes the
// preset decimal upward; Custom reveals the free-text decimal input so off-grid
// purities (and the 0.999-vs-0.9999 distinction) stay reachable. The parent
// keeps owning the decimal-string `value`, like RiskProfileSelect. The prefix
// scopes the id/testids so Create + Edit instances don't collide.

const CUSTOM = "custom";

type Props = {
  /** Disambiguating prefix for the input id, e.g. "gold_create". */
  idPrefix: string;
  /** Purity as a decimal-fraction string, e.g. "0.9999". */
  value: string;
  onChange: (next: string) => void;
};

export function GoldPuritySelect({ idPrefix, value, onChange }: Props) {
  const { t } = useTranslation("investments");
  const id = `${idPrefix}_purity`;
  // Mode is held locally so the user can pick Custom even while the current
  // value still happens to match a preset; the dropdown wouldn't otherwise let
  // them leave a preset row.
  const [mode, setMode] = useState<number | typeof CUSTOM>(
    () => goldPurityPresetKarat(value) ?? CUSTOM,
  );

  function handleSelect(next: string) {
    if (next === CUSTOM) {
      setMode(CUSTOM);
      return;
    }
    const karat = Number(next);
    setMode(karat);
    const preset = GOLD_PURITY_PRESETS.find((p) => p.karat === karat);
    if (preset) onChange(preset.value);
  }

  return (
    <div className="grid gap-2">
      <Label htmlFor={id}>{t("gold.fields.purity")}</Label>
      <select
        id={id}
        data-testid={`${idPrefix}-purity-select`}
        className="h-9 rounded-md border border-input bg-background px-3 text-sm"
        value={mode === CUSTOM ? CUSTOM : String(mode)}
        onChange={(e) => handleSelect(e.target.value)}
      >
        {GOLD_PURITY_PRESETS.map((p) => (
          <option key={p.karat} value={String(p.karat)}>
            {t("gold.purity.karatOption", { karat: p.karat })}
          </option>
        ))}
        <option value={CUSTOM}>{t("gold.purity.customOption")}</option>
      </select>
      {mode === CUSTOM && (
        <Input
          required
          inputMode="decimal"
          data-testid={`${idPrefix}-purity-custom`}
          aria-label={t("gold.purity.customLabel")}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          placeholder={t("gold.placeholders.purity")}
        />
      )}
    </div>
  );
}
