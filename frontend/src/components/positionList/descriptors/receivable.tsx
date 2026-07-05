// Receivable, on the non-investment preset (ADR-0043). Two group-specific bits
// beyond the shared surface: the secondary line carries the counterparty + an
// optional due-date suffix, and a value-over-time chart per currency renders
// beneath the headline (epic #204) — supplied through the preset's
// `renderHeadlineExtra` slot, keeping the core untouched.
/* eslint-disable react-refresh/only-export-components */
import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { SnapshotChart } from "@/components/SnapshotChart";
import { CreateReceivableDialog } from "@/components/CreateReceivableDialog";
import { EditReceivableDialog } from "@/components/EditReceivableDialog";
import {
  useReceivables,
  useDeleteReceivable,
  useImportCreateReceivable,
} from "@/hooks/useReceivables";
import { useReceivableTimeSeries } from "@/hooks/useReceivableTimeSeries";
import { nonInvestmentDescriptor } from "@/components/positionList/presets/nonInvestment";
import { aggregateListPositions } from "@/lib/listAggregates";
import { formatDate } from "@/lib/format";
import type { ReceivableListItem } from "@/api/types";

// Total-outstanding over time, per native currency. Receivables are a flat
// value-only group (no cost basis), so aggregateListPositions runs with cost
// omitted; a currency with fewer than two points is skipped.
function ReceivableCharts({ items }: { items: ReceivableListItem[] }) {
  const { t } = useTranslation("receivables");
  const timeSeries = useReceivableTimeSeries();
  const timeSeriesByCurrency = useMemo(() => {
    const positions = items.map((item) => ({
      id: item.receivable.id,
      currency: item.receivable.native_currency,
      status: item.receivable.status,
      terminated_at: item.receivable.terminated_at,
      latestValue: item.latest_snapshot
        ? Number(item.latest_snapshot.amount)
        : null,
      snapshots: timeSeries.byId.get(item.receivable.id)?.snapshots ?? [],
    }));
    return aggregateListPositions(positions).timeSeriesByCurrency;
  }, [items, timeSeries.byId]);

  const chartCurrencies = [...timeSeriesByCurrency.keys()].sort();
  return (
    <>
      {chartCurrencies.map((currency) => {
        const series = timeSeriesByCurrency.get(currency) ?? [];
        if (series.length < 2) return null;
        return (
          <Card
            key={currency}
            data-testid={`receivables-value-chart-${currency}`}
          >
            <CardHeader>
              <CardTitle>{t("home.valueChartTitle")}</CardTitle>
              <CardDescription>
                {t("home.valueChartDescription", { currency })}
              </CardDescription>
            </CardHeader>
            <CardContent>
              <SnapshotChart
                snapshots={series.map((p) => ({
                  year_month: p.year_month,
                  amount: String(p.value),
                }))}
                currency={currency}
              />
            </CardContent>
          </Card>
        );
      })}
    </>
  );
}

export const receivableDescriptor = nonInvestmentDescriptor<ReceivableListItem>(
  {
    entityKey: "receivable",
    testIdPrefix: "receivable",
    group: "receivables",
    i18nNamespaces: ["receivables", "common", "errors"],
    keys: {
      listTitle: "receivables:listTitle",
      listSubtitle: "receivables:listSubtitle",
      emptyTitle: "receivables:emptyTitle",
      emptyBody: "receivables:emptyBody",
      noun: "receivables:noun",
      nounPlural: "receivables:nounPlural",
      valueLabel: "receivables:sortLatestBalance",
      rowActions: "receivables:rowActions",
      deleteTitle: "receivables:deleteTitle",
    },
    useList: useReceivables,
    useDelete: useDeleteReceivable,
    useImport: useImportCreateReceivable,
    entity: (item) => item.receivable,
    getSnapshot: (item) => item.latest_snapshot,
    getSecondary: (item, t) =>
      item.receivable.counterparty_name +
      (item.receivable.due_date
        ? t("receivables:rowDueSuffix", {
            date: formatDate(item.receivable.due_date),
          })
        : ""),
    deleteDescription: (item, t) =>
      t("receivables:deleteRowDescription", {
        name: item.receivable.display_name,
      }),
    headlineLabelKey: "receivables:totalOutstanding",
    headlineTestId: "receivables-total",
    renderHeadlineExtra: (items) => <ReceivableCharts items={items} />,
    renderCreateDialog: () => <CreateReceivableDialog />,
    renderEditDialog: (item, props) => (
      <EditReceivableDialog
        key={item.receivable.id}
        receivable={item.receivable}
        {...props}
      />
    ),
  },
);
