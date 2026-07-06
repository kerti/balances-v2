// Investment-list-screen time graph (issue #14, slice 14c). One Card +
// chart per currency, each showing aggregated value (Area) + cost
// (Line) across all active positions in the subtype that share that
// currency. Built on the same lazy-loaded SnapshotChart that the
// detail screens use — the chart impl already understands the
// costSeries prop.
//
// Renders nothing when the aggregate has no monthly data yet (e.g.
// brand-new household with snapshots only this month); the chart's
// own length-2 minimum on `SnapshotChart` handles the "not enough
// data" case per currency.

import { useTranslation } from "react-i18next";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { SnapshotChart } from "@/components/SnapshotChart";
import type { TimePoint } from "@/lib/listAggregates";

type Props = {
  timeSeriesByCurrency: Map<string, TimePoint[]>;
};

export function ListTimeGraph({ timeSeriesByCurrency }: Props) {
  const { t } = useTranslation("investments");
  if (timeSeriesByCurrency.size === 0) return null;
  return (
    <>
      {[...timeSeriesByCurrency.entries()].map(([currency, series]) => (
        <Card key={currency} data-testid={`list-time-graph-${currency}`}>
          <CardHeader>
            <CardTitle>{t("list.chartTitle")}</CardTitle>
            <CardDescription>{t("list.chartDescription", { currency })}</CardDescription>
          </CardHeader>
          <CardContent>
            <SnapshotChart
              snapshots={series.map((p) => ({
                year_month: p.year_month,
                amount: String(p.value),
              }))}
              costSeries={series.map((p) => ({
                year_month: p.year_month,
                cost: p.cost,
              }))}
              currency={currency}
            />
          </CardContent>
        </Card>
      ))}
    </>
  );
}
