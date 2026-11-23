import { useState } from "react";
import { PageHeader } from "@/components/common/PageHeader";
import { useTranslation } from "react-i18next";
import { MissingFxWarning } from "@/components/common/MissingFxWarning";
import { RatesLine } from "@/components/common/RatesLine";
import { TagsCard } from "@/components/common/TagsCard";
import { TagDialog } from "@/components/dialogs/TagDialog";
import { TagBreakdownSection } from "@/components/tags/TagBreakdownSection";
import { useFxRates } from "@/hooks/useFxRates";
import { useTags, useTagBreakdown } from "@/hooks/useTags";
import { cellKey, projectTagBreakdown } from "@/lib/tagBreakdown";

// TagsScreen is the breakdown report (ADR-0028) plus tag maintenance — merged
// onto one page rather than splitting management into a Settings subpage, since
// a "manage" surface and its own report were needless duplication when they
// lived in two places. The report leads (one donut + holdings/liabilities/net
// per tag in the reporting currency, with an Untagged bucket, the rates line and
// the missing-rate warning as on the dashboard — #937), and tag management sits
// at the bottom — the report is what a returning household member comes for;
// creating a tag is the occasional setup task. Per ADR-0050 (#509) the report is
// the container/single-source-of-truth: it owns the query and the pie-inclusion
// state, and hands the projection to TagBreakdownSection, which picks the wide
// table (desktop) vs stacked cards (phones) at 768px. Management stays one
// responsive card (it reflows without breaking the a11y floor).
export function TagsScreen() {
  const { t } = useTranslation(["tags", "common"]);
  const { data: tags } = useTags();
  const { data: report, isLoading } = useTagBreakdown();
  const { data: rates } = useFxRates();
  // null = every slice included (the default); a Set once the user toggles.
  const [checked, setChecked] = useState<Set<string> | null>(null);

  const bd = report && tags ? projectTagBreakdown(report, tags, t("report.untagged")) : null;
  const hasCells = bd !== null && bd.cells.length > 0;

  function isChecked(key: string) {
    return checked === null || checked.has(key);
  }

  function toggle(key: string) {
    if (!bd) return;
    const next = new Set(checked ?? bd.cells.map(cellKey));
    if (next.has(key)) next.delete(key);
    else next.add(key);
    setChecked(next);
  }

  const missing = report?.missing_fx ?? [];
  const notes =
    report && report.year_month ? (
      <>
        <RatesLine
          fxRatesUsed={report.fx_rates_used}
          yearMonth={report.year_month}
          currency={report.reporting_currency}
          rates={rates ?? []}
          testIdPrefix="tag-breakdown"
        />
        <MissingFxWarning missingFx={missing} yearMonth={report.year_month} scope="groupTotal" />
      </>
    ) : null;

  return (
    <div className="space-y-6">
      <PageHeader
        variant="hub"
        title={t("report.title")}
        subtitle={t("report.subtitle")}
        actions={<TagDialog mode="create" />}
      />

      {!isLoading && !hasCells && (
        <p className="text-sm text-muted-foreground" data-testid="tags-empty">
          {t("report.empty")}
        </p>
      )}

      {hasCells && (
        <TagBreakdownSection bd={bd} notes={notes} isChecked={isChecked} toggle={toggle} />
      )}

      <TagsCard />
    </div>
  );
}
