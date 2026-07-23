import { useState } from "react";
import { useTranslation } from "react-i18next";
import { TagsCard } from "@/components/common/TagsCard";
import { TagBreakdownSection } from "@/components/tags/TagBreakdownSection";
import { useTags, useTagBreakdown } from "@/hooks/useTags";
import { aggregateTagBreakdown, cellKey, type TagCell } from "@/lib/tagBreakdown";

// TagsScreen is the breakdown report (ADR-0028) plus tag maintenance — merged
// onto one page rather than splitting management into a Settings subpage, since
// a "manage" surface and its own report were needless duplication when they
// lived in two places. The report leads (donut + holdings/liabilities/net per
// currency, with an Untagged bucket, no FX), and tag management sits at the
// bottom — the report is what a returning household member comes for; creating
// a tag is the occasional setup task. Per ADR-0050 (#509) the report is the
// container/single-source-of-truth: it owns the query and the per-currency
// pie-inclusion state, and hands each currency to TagBreakdownSection, which
// picks the wide table (desktop) vs stacked cards (phones) at 768px. Management
// stays one responsive card (it reflows without breaking the a11y floor).
export function TagsScreen() {
  const { t } = useTranslation(["tags", "common"]);
  const { data: tags } = useTags();
  const { data: rows, isLoading } = useTagBreakdown();
  const [checked, setChecked] = useState<Record<string, Set<string>>>({});

  const untaggedLabel = t("report.untagged");
  const breakdowns = rows && tags ? aggregateTagBreakdown(rows, tags, untaggedLabel) : [];

  function isChecked(currency: string, key: string) {
    return checked[currency]?.has(key) ?? true;
  }

  function toggle(currency: string, key: string, cells: TagCell[]) {
    setChecked((prev) => {
      const current = prev[currency] ?? new Set(cells.map(cellKey));
      const next = new Set(current);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return { ...prev, [currency]: next };
    });
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">{t("report.title")}</h1>
        <p className="text-sm text-muted-foreground">{t("report.subtitle")}</p>
      </div>

      {!isLoading && breakdowns.length === 0 && (
        <p className="text-sm text-muted-foreground" data-testid="tags-empty">
          {t("report.empty")}
        </p>
      )}

      {breakdowns.map((bd) => (
        <TagBreakdownSection
          key={bd.currency}
          bd={bd}
          isChecked={(key) => isChecked(bd.currency, key)}
          toggle={(key) => toggle(bd.currency, key, bd.cells)}
        />
      ))}

      <TagsCard />
    </div>
  );
}
