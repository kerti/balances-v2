// Liabilities landing page (epic #204, slice 2) — the value-only parity twin
// of InvestmentsHome / AssetsHome.
//
// Aggregates across the two liability subtypes (personal / institutional) into
// one card set:
//   1. Total owed headline (no cost basis — liabilities have no ledger).
//   2. Total owed over time.
//   3. 100%-stacked category share over time.
//   4. Current category-mix pie (2-way, kept for structural parity per the #204
//      grill decision). A composition chart with a single category is not drawn.
//
// A multi-currency household holding a foreign currency gets **one set in the
// reporting currency**, from the dashboard's own engine pass (`useHubConversion`
// → /api/reports/groups/liability; ADR-0002 exclude-and-warn, ADR-0022
// amendment, #845), with native subtotals as a muted line. Otherwise each
// currency renders its own card-set (14c convention): headline + pie
// active-only, the time + stack series include terminated positions
// historically (capped at terminated_at), via the shared `aggregateGroupHome`.

import { PageHeader } from "@/components/common/PageHeader";
import { useMemo } from "react";
import { Link } from "react-router";
import { CalendarPlus } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { routes } from "@/lib/routes";
import { SnapshotChart } from "@/components/charts/SnapshotChart";
import {
  GroupCategoryStackChart,
  type GroupStackCategory,
} from "@/components/charts/GroupCategoryStackChart";
import { InvestmentPieChart, type PieSlice } from "@/components/charts/InvestmentPieChart";
import { useLiabilities } from "@/hooks/useLiabilities";
import { useLiabilityTimeSeries } from "@/hooks/useLiabilityTimeSeries";
import { useHubConversion } from "@/hooks/useGroupBreakdown";
import { ConvertedHeadline } from "@/components/common/ConvertedHeadline";
import { MissingFxWarning } from "@/components/common/MissingFxWarning";
import {
  aggregateGroupHome,
  type GroupCategoryTimePoint,
  type GroupPosition,
} from "@/lib/groupHomeAggregates";
import {
  breakdownCategorySeries,
  breakdownCategorySlices,
  breakdownValueSeries,
  categoriesHeld,
  latestMonth,
  slicesHeld,
} from "@/lib/groupBreakdown";
import { formatCurrency } from "@/lib/format";
import { LIABILITY_CATEGORY_COLORS } from "@/lib/categoryColors";
import { headlineSurface } from "@/lib/headline";
import { cn } from "@/lib/utils";

type LiabilityCategory = "personal" | "institutional";

const LIABILITY_CATEGORIES: LiabilityCategory[] = ["personal", "institutional"];

export function LiabilitiesHome() {
  const { t } = useTranslation(["common", "liabilities", "errors"]);
  const liabilities = useLiabilities();
  const timeSeries = useLiabilityTimeSeries();

  const positions = useMemo<GroupPosition[]>(() => {
    const out: GroupPosition[] = [];
    for (const it of liabilities.data ?? []) {
      const ts = timeSeries.byId.get(it.liability.id);
      out.push({
        id: it.liability.id,
        currency: it.liability.native_currency,
        status: it.liability.status,
        terminated_at: it.liability.terminated_at,
        latestValue: it.latest_snapshot ? Number(it.latest_snapshot.amount) : null,
        snapshots: ts?.snapshots ?? [],
        category: it.liability.subtype,
      });
    }
    return out;
  }, [liabilities.data, timeSeries.byId]);

  const aggregates = useMemo(
    () => aggregateGroupHome(positions, LIABILITY_CATEGORIES),
    [positions],
  );

  const currencies = aggregates.byCurrency.map((c) => c.currency);
  const { converted, reporting, breakdown } = useHubConversion("liability", currencies);

  const cardSets: CardSet[] =
    converted && breakdown.data
      ? [
          {
            currency: reporting,
            value: breakdownValueSeries(breakdown.data),
            stack: breakdownCategorySeries(breakdown.data, LIABILITY_CATEGORIES),
            slices: buildCategorySlices(
              breakdownCategorySlices(breakdown.data, LIABILITY_CATEGORIES),
              t,
            ),
          },
        ]
      : converted
        ? []
        : currencies.map((currency) => ({
            currency,
            value: aggregates.timeSeriesByCurrency.get(currency) ?? [],
            stack: aggregates.categorySeriesByCurrency.get(currency) ?? [],
            slices: buildCategorySlices(aggregates.categoryPieByCurrency.get(currency) ?? [], t),
          }));
  const latest = latestMonth(breakdown.data);

  const stackCategories: GroupStackCategory[] = LIABILITY_CATEGORIES.map((c) => ({
    key: c,
    label: t(`liabilities:home.categoryLabel.${c}`),
    color: LIABILITY_CATEGORY_COLORS[c],
  }));

  return (
    <div className="space-y-6" data-testid="liabilities-home">
      <PageHeader
        variant="hub"
        title={t("common:home.liabilities.title")}
        subtitle={t("liabilities:home.subtitle")}
        actions={
          <Button asChild data-testid="liabilities-enter-month">
            <Link to={routes.liabilitiesEnter}>
              <CalendarPlus className="mr-1 size-4" />
              {t("common:bulkEntry.trigger")}
            </Link>
          </Button>
        }
      />

      {(liabilities.isPending || (converted && breakdown.isPending)) && (
        <p className="text-sm text-muted-foreground">{t("common:loading")}</p>
      )}

      {(liabilities.error ?? breakdown.error) && (
        <p className="text-sm text-destructive">
          {t("errors:failedToLoad", {
            message: ((liabilities.error ?? breakdown.error) as Error).message,
          })}
        </p>
      )}

      {converted ? (
        latest && (
          <ConvertedHeadline
            label={t("liabilities:home.totalOwedTitle")}
            total={latest.total}
            currency={reporting}
            nativeSubtotals={aggregates.byCurrency.filter((c) => c.currency !== reporting)}
            countText={t("liabilities:home.totalOwedCount", { count: aggregates.count })}
            testId="home-total"
          >
            <MissingFxWarning
              missingFx={latest.missing_fx}
              yearMonth={latest.year_month}
              scope="groupTotal"
            />
          </ConvertedHeadline>
        )
      ) : (
        <TotalOwedCard aggregates={aggregates.byCurrency} count={aggregates.count} />
      )}

      {cardSets.map(({ currency, value, stack, slices }) => (
        <div key={currency} className="space-y-4">
          <ValueCard currency={currency} series={value} />
          <CategoryStackCard currency={currency} series={stack} categories={stackCategories} />
          <CategoryPieCard currency={currency} slices={slices} />
        </div>
      ))}
    </div>
  );
}

// One chart set: a single currency's cards from the native lists, or the
// reporting-currency set converted by the backend.
type CardSet = {
  currency: string;
  value: { year_month: string; value: number }[];
  stack: GroupCategoryTimePoint[];
  slices: PieSlice[];
};

type TFn = (key: string, opts?: Record<string, unknown>) => string;

function buildCategorySlices(pie: { category: string; value: number }[], t: TFn): PieSlice[] {
  return LIABILITY_CATEGORIES.map((c) => {
    const found = pie.find((p) => p.category === c);
    return {
      key: c,
      label: t(`liabilities:home.categoryLabel.${c}`),
      value: found?.value ?? 0,
      color: LIABILITY_CATEGORY_COLORS[c],
    };
  });
}

function TotalOwedCard({
  aggregates,
  count,
}: {
  aggregates: { currency: string; value: number }[];
  count: number;
}) {
  const { t } = useTranslation("liabilities");
  if (aggregates.length === 0) return null;
  const multi = aggregates.length > 1;
  return (
    <div className={cn("rounded-lg border p-4", headlineSurface)} data-testid="home-total">
      <div className="text-sm text-muted-foreground">{t("home.totalOwedTitle")}</div>
      <div className="mt-0.5 text-2xl font-semibold">
        {aggregates.map((a, i) => (
          // Multi-currency stacks one figure per line on mobile (ADR-0050) —
          // additional currencies drop below the primary instead of running off
          // to its right; single-currency renders inline unchanged.
          <span key={a.currency} className={multi ? "block md:inline" : undefined}>
            {i > 0 && <span className="mx-2 hidden text-muted-foreground md:inline">·</span>}
            {formatCurrency(String(a.value), a.currency)}
          </span>
        ))}
      </div>
      <div className="mt-1 text-xs text-muted-foreground">
        {t("home.totalOwedCount", { count })}
      </div>
    </div>
  );
}

function ValueCard({
  currency,
  series,
}: {
  currency: string;
  series: { year_month: string; value: number }[];
}) {
  const { t } = useTranslation("liabilities");
  if (series.length < 2) return null;
  return (
    <Card data-testid={`home-value-${currency}`}>
      <CardHeader>
        <CardTitle>{t("home.valueChartTitle")}</CardTitle>
        <CardDescription>{t("home.valueChartDescription", { currency })}</CardDescription>
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
}

function CategoryStackCard({
  currency,
  series,
  categories,
}: {
  currency: string;
  series: Parameters<typeof GroupCategoryStackChart>[0]["series"];
  categories: GroupStackCategory[];
}) {
  const { t } = useTranslation("liabilities");
  if (series.length < 2 || categoriesHeld(series) < 2) return null;
  return (
    <Card data-testid={`home-category-stack-${currency}`}>
      <CardHeader>
        <CardTitle>{t("home.categoryStackTitle")}</CardTitle>
        <CardDescription>{t("home.categoryStackDescription", { currency })}</CardDescription>
      </CardHeader>
      <CardContent>
        <GroupCategoryStackChart series={series} categories={categories} currency={currency} />
      </CardContent>
    </Card>
  );
}

function CategoryPieCard({ currency, slices }: { currency: string; slices: PieSlice[] }) {
  const { t } = useTranslation("liabilities");
  if (slicesHeld(slices) < 2) return null;
  return (
    <Card data-testid={`home-category-pie-${currency}`}>
      <CardHeader>
        <CardTitle>{t("home.categoryPieTitle")}</CardTitle>
        <CardDescription>{t("home.categoryPieDescription", { currency })}</CardDescription>
      </CardHeader>
      <CardContent>
        <InvestmentPieChart slices={slices} currency={currency} />
      </CardContent>
    </Card>
  );
}
