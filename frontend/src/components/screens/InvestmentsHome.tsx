// Investments landing page (issue #14 slice 14d).
//
// Aggregates across all 5 subtypes (stock / mutual fund / bond / time
// deposit / gold) into one card set:
//   1. Cross-subtype Value / Cost / P/L headline.
//   2. Value + cost over time.
//   3. 100% stacked category share over time.
//   4. Two pies side-by-side — category share + risk profile share.
// A composition chart with a single category is not drawn.
//
// A multi-currency household holding a foreign currency gets **one set in the
// reporting currency**, from the dashboard's own engine pass (`useHubConversion`
// → /api/reports/groups/investment; ADR-0002 exclude-and-warn, ADR-0022
// amendment, #845), with native subtotals as a muted line. That set is
// value-only: cost basis and P/L stay native, on the per-subtype list screens.
// Otherwise each currency renders its own set from the native lists (14c
// convention), with cost and P/L.
//
// **Active-only (native sets).** Terminated positions drop out of every output, same
// as `aggregateListPositions`.
//
// Headline cost basis rides on each list payload (item.cost_basis, #18); the
// per-month value + cost series for the charts come from one household-scoped
// fetch (`useInvestmentTimeSeries`, #22) — no per-position fan-out.

import { PageHeader } from "@/components/common/PageHeader";
import { useMemo } from "react";
import { Link } from "react-router";
import { CalendarPlus } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { INVESTMENT_CATEGORY_COLORS, RISK_PROFILE_COLORS } from "@/lib/categoryColors";
import { routes } from "@/lib/routes";
import { InvestmentListHeadline } from "@/components/common/InvestmentListHeadline";
import { SnapshotChart } from "@/components/charts/SnapshotChart";
import { CategoryStackChart } from "@/components/charts/CategoryStackChart";
import { InvestmentPieChart, type PieSlice } from "@/components/charts/InvestmentPieChart";
import {
  useBonds,
  useGolds,
  useMutualFunds,
  useStocks,
  useTimeDeposits,
} from "@/hooks/useInvestments";
import { useInvestmentTimeSeries } from "@/hooks/useInvestmentTimeSeries";
import { useHubConversion } from "@/hooks/useGroupBreakdown";
import { ConvertedHeadline } from "@/components/common/ConvertedHeadline";
import { MissingFxWarning } from "@/components/common/MissingFxWarning";
import {
  breakdownCategorySeries,
  breakdownCategorySlices,
  breakdownRiskSlices,
  breakdownValueSeries,
  categoriesHeld,
  latestMonth,
  slicesHeld,
} from "@/lib/groupBreakdown";
import {
  aggregateHomePositions,
  INVESTMENT_CATEGORIES,
  INVESTMENT_RISK_PROFILES,
  type CategoryTimePoint,
  type HomePosition,
  type InvestmentCategory,
  type InvestmentRiskProfile,
} from "@/lib/homeAggregates";
import type { Investment, InvestmentSnapshot } from "@/api/types";

// The common subset of every subtype's *ListItem the home aggregation reads —
// lets one `push` helper map all five lists (each adds its own `details`).
type HomeListItem = {
  investment: Investment;
  latest_snapshot: InvestmentSnapshot | null;
  cost_basis: string;
};

// Risk pie: semantic traffic-light. Low = emerald (matches the gain
// tone), medium = amber, high = a red close to the `--destructive`
// token (the actual CSS var is OKLCH so we use a Tailwind red-500
// hex here for a stable static-fill that recharts can consume).
export function InvestmentsHome() {
  const { t } = useTranslation(["common", "investments", "errors"]);
  const stocks = useStocks();
  const mutualFunds = useMutualFunds();
  const bonds = useBonds();
  const timeDeposits = useTimeDeposits();
  const golds = useGolds();

  const noun = t("investments:list.noun");
  const nounPlural = t("investments:list.nounPlural");

  const timeSeries = useInvestmentTimeSeries();

  // Each subtype's list items map to the same HomePosition shape — headline
  // cost from item.cost_basis (#18), value + cost series from the time-series
  // fetch (#22). Only the category literal differs.
  const positions = useMemo<HomePosition[]>(() => {
    const out: HomePosition[] = [];
    const push = (items: HomeListItem[] | undefined, category: InvestmentCategory) => {
      for (const it of items ?? []) {
        const ts = timeSeries.byId.get(it.investment.id);
        out.push({
          id: it.investment.id,
          currency: it.investment.native_currency,
          status: it.investment.status,
          terminated_at: it.investment.terminated_at,
          latestValue: it.latest_snapshot ? Number(it.latest_snapshot.amount) : null,
          cost: Number(it.cost_basis),
          snapshots: ts?.snapshots ?? [],
          costSeries: ts?.costSeries ?? [],
          category,
          riskProfile: it.investment.risk_profile,
        });
      }
    };
    push(stocks.data, "stock");
    push(mutualFunds.data, "mutualFund");
    push(bonds.data, "bond");
    push(timeDeposits.data, "timeDeposit");
    push(golds.data, "gold");
    return out;
  }, [stocks.data, mutualFunds.data, bonds.data, timeDeposits.data, golds.data, timeSeries.byId]);

  const aggregates = useMemo(() => aggregateHomePositions(positions), [positions]);

  const currencies = aggregates.byCurrency.map((c) => c.currency);
  const { converted, reporting, breakdown } = useHubConversion("investment", currencies);

  const anyPending =
    stocks.isPending ||
    mutualFunds.isPending ||
    bonds.isPending ||
    timeDeposits.isPending ||
    golds.isPending ||
    (converted && breakdown.isPending);
  const firstError =
    stocks.error ??
    mutualFunds.error ??
    bonds.error ??
    timeDeposits.error ??
    golds.error ??
    breakdown.error;

  const cardSets: CardSet[] =
    converted && breakdown.data
      ? [
          {
            currency: reporting,
            value: breakdownValueSeries(breakdown.data),
            stack: breakdownCategorySeries(breakdown.data, INVESTMENT_CATEGORIES),
            categorySlices: buildCategorySlices(
              breakdownCategorySlices(breakdown.data, INVESTMENT_CATEGORIES),
              t,
            ),
            riskSlices: buildRiskSlices(
              breakdownRiskSlices(breakdown.data, INVESTMENT_RISK_PROFILES),
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
            categorySlices: buildCategorySlices(
              aggregates.categoryPieByCurrency.get(currency) ?? [],
              t,
            ),
            riskSlices: buildRiskSlices(aggregates.riskPieByCurrency.get(currency) ?? [], t),
          }));
  const latest = latestMonth(breakdown.data);

  return (
    <div className="space-y-6" data-testid="investments-home">
      {/* Two bulk monthly-entry actions (ADR-0046): qty×price prices for
          Stock/MutualFund/Gold (#423) and accrued values for Bond/TimeDeposit
          (#424) — one per snapshot shape, matching how statements arrive. */}
      <PageHeader
        variant="hub"
        title={t("common:home.investments.title")}
        subtitle={t("investments:home.subtitle")}
        actions={
          <>
            <Button asChild variant="outline" data-testid="investments-enter-accrued">
              <Link to={routes.investmentsEnterAccrued}>
                <CalendarPlus className="mr-1 size-4" />
                {t("common:bulkEntry.investmentsAccrued.trigger")}
              </Link>
            </Button>
            <Button asChild data-testid="investments-enter-prices">
              <Link to={routes.investmentsEnterPrices}>
                <CalendarPlus className="mr-1 size-4" />
                {t("common:bulkEntry.investments.trigger")}
              </Link>
            </Button>
          </>
        }
      />

      {anyPending && <p className="text-sm text-muted-foreground">{t("common:loading")}</p>}

      {firstError && (
        <p className="text-sm text-destructive">
          {t("errors:failedToLoad", {
            message: (firstError as Error).message,
          })}
        </p>
      )}

      {converted ? (
        latest && (
          <ConvertedHeadline
            label={t("investments:list.totalValue")}
            total={latest.total}
            currency={reporting}
            nativeSubtotals={aggregates.byCurrency.filter((c) => c.currency !== reporting)}
            countText={t("common:list.activeCount", {
              count: aggregates.count,
              noun: aggregates.count === 1 ? noun : nounPlural,
            })}
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
        <InvestmentListHeadline
          aggregates={aggregates.byCurrency}
          count={aggregates.count}
          noun={noun}
          nounPlural={nounPlural}
          testId="home-total"
        />
      )}

      {cardSets.map(({ currency, value, stack, categorySlices, riskSlices }) => (
        <div key={currency} className="space-y-4">
          <ValueCostCard currency={currency} series={value} />
          <CategoryStackCard currency={currency} series={stack} />
          <div className="grid gap-4 md:grid-cols-2">
            <CategoryPieCard currency={currency} slices={categorySlices} />
            <RiskPieCard currency={currency} slices={riskSlices} />
          </div>
        </div>
      ))}
    </div>
  );
}

// One chart set: a single currency's cards from the native lists (with cost), or
// the reporting-currency set converted by the backend (value only).
type CardSet = {
  currency: string;
  value: { year_month: string; value: number; cost?: number }[];
  stack: CategoryTimePoint[];
  categorySlices: PieSlice[];
  riskSlices: PieSlice[];
};

type TFn = (key: string, opts?: Record<string, unknown>) => string;

function buildCategorySlices(
  pie: { category: InvestmentCategory; value: number }[],
  t: TFn,
): PieSlice[] {
  return INVESTMENT_CATEGORIES.map((c) => {
    const found = pie.find((p) => p.category === c);
    return {
      key: c,
      label: t(`investments:home.categoryLabel.${c}`),
      value: found?.value ?? 0,
      color: INVESTMENT_CATEGORY_COLORS[c],
    };
  });
}

function buildRiskSlices(
  pie: { profile: InvestmentRiskProfile; value: number }[],
  t: TFn,
): PieSlice[] {
  return INVESTMENT_RISK_PROFILES.map((r) => {
    const found = pie.find((p) => p.profile === r);
    const labelKey = `investments:riskProfile.badge${r[0].toUpperCase()}${r.slice(1)}`;
    return {
      key: r,
      label: t(labelKey),
      value: found?.value ?? 0,
      color: RISK_PROFILE_COLORS[r],
    };
  });
}

function ValueCostCard({
  currency,
  series,
}: {
  currency: string;
  series: { year_month: string; value: number; cost?: number }[];
}) {
  const { t } = useTranslation("investments");
  if (series.length < 2) return null;
  const withCost = series.every((p) => p.cost !== undefined);
  return (
    <Card data-testid={`home-value-cost-${currency}`}>
      <CardHeader>
        <CardTitle>{t(withCost ? "home.valueCostChartTitle" : "home.valueChartTitle")}</CardTitle>
        <CardDescription>
          {t(withCost ? "home.valueCostChartDescription" : "home.valueChartDescription", {
            currency,
          })}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <SnapshotChart
          snapshots={series.map((p) => ({
            year_month: p.year_month,
            amount: String(p.value),
          }))}
          costSeries={
            withCost
              ? series.map((p) => ({ year_month: p.year_month, cost: p.cost ?? 0 }))
              : undefined
          }
          currency={currency}
        />
      </CardContent>
    </Card>
  );
}

function CategoryStackCard({
  currency,
  series,
}: {
  currency: string;
  series: Parameters<typeof CategoryStackChart>[0]["series"];
}) {
  const { t } = useTranslation("investments");
  if (series.length < 2 || categoriesHeld(series) < 2) return null;
  return (
    <Card data-testid={`home-category-stack-${currency}`}>
      <CardHeader>
        <CardTitle>{t("home.categoryStackTitle")}</CardTitle>
        <CardDescription>{t("home.categoryStackDescription", { currency })}</CardDescription>
      </CardHeader>
      <CardContent>
        <CategoryStackChart series={series} currency={currency} />
      </CardContent>
    </Card>
  );
}

function CategoryPieCard({ currency, slices }: { currency: string; slices: PieSlice[] }) {
  const { t } = useTranslation("investments");
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

function RiskPieCard({ currency, slices }: { currency: string; slices: PieSlice[] }) {
  const { t } = useTranslation("investments");
  if (slicesHeld(slices) < 2) return null;
  return (
    <Card data-testid={`home-risk-pie-${currency}`}>
      <CardHeader>
        <CardTitle>{t("home.riskPieTitle")}</CardTitle>
        <CardDescription>{t("home.riskPieDescription", { currency })}</CardDescription>
      </CardHeader>
      <CardContent>
        <InvestmentPieChart slices={slices} currency={currency} ordered />
      </CardContent>
    </Card>
  );
}
