import { useTranslation } from "react-i18next";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { InvestmentPieChart, type PieSlice } from "@/components/charts/InvestmentPieChart";
import { TagBreakdownTable } from "@/components/tags/TagBreakdownTable";
import { TagBreakdownCards } from "@/components/tags/TagBreakdownCards";
import { useIsMobile } from "@/hooks/use-mobile";
import { cellKey, type CurrencyBreakdown } from "@/lib/tagBreakdown";

type Props = {
  bd: CurrencyBreakdown;
  isChecked: (key: string) => boolean;
  toggle: (key: string) => void;
};

// The Tag breakdown's mobile–web split point (ADR-0050 B2c, #509). TagsScreen
// owns the query and the per-currency pie-inclusion state; this section takes
// one currency's projection and picks which renderer mounts — stacked cards on
// phones, the wide table on desktop (`useIsMobile`, 768px) — so only one tree is
// ever in the DOM. The pie is shared: its legend sits to the right on desktop,
// and is dropped entirely on phones — the breakdown cards below double as the
// legend (each carries the tag badge + colour), so a mobile legend is just
// clutter, and its absence lets the donut's container tighten. The per-currency
// `tag-breakdown-<currency>` testid wraps whichever renderer is active.
export function TagBreakdownSection({ bd, isChecked, toggle }: Props) {
  const { t } = useTranslation(["tags"]);
  const isMobile = useIsMobile();

  const slices: PieSlice[] = bd.cells
    .filter((c) => c.holdings > 0 && isChecked(cellKey(c)))
    .map((c) => ({
      key: cellKey(c),
      label: c.name,
      value: c.holdings,
      color: c.color,
    }));

  return (
    <Card data-testid={`tag-breakdown-${bd.currency}`}>
      <CardHeader>
        <CardTitle className="text-base">
          {t("report.currencyHeading", { currency: bd.currency })}
        </CardTitle>
      </CardHeader>
      <CardContent className={isMobile ? "space-y-3" : "space-y-6"}>
        <InvestmentPieChart
          slices={slices}
          currency={bd.currency}
          legendPosition={isMobile ? "none" : "right"}
        />

        {isMobile ? (
          <TagBreakdownCards bd={bd} isChecked={isChecked} toggle={toggle} />
        ) : (
          <TagBreakdownTable bd={bd} isChecked={isChecked} toggle={toggle} />
        )}
      </CardContent>
    </Card>
  );
}
