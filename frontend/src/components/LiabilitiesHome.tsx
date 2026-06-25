// Liabilities landing page (epic #204, slice 2) — the value-only parity twin
// of InvestmentsHome / AssetsHome.
//
// Aggregates across the two liability subtypes (personal / institutional) into
// one set of per-currency cards:
//   1. Total owed headline (no cost basis — liabilities have no ledger).
//   2. Total owed over time (one line per currency).
//   3. 100%-stacked category share over time (one card per currency).
//   4. Current category-mix pie (one per currency — 2-way, kept for structural
//      parity per the #204 grill decision).
//
// **No FX.** Each currency renders its own card-set (14c convention). Headline
// + pie are active-only; the time + stack series include terminated positions
// historically (capped at terminated_at), via the shared `aggregateGroupHome`.

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
import {
  GroupCategoryStackChart,
  type GroupStackCategory,
} from "@/components/GroupCategoryStackChart";
import {
  InvestmentPieChart,
  type PieSlice,
} from "@/components/InvestmentPieChart";
import { useLiabilities } from "@/hooks/useLiabilities";
import { useLiabilityTimeSeries } from "@/hooks/useLiabilityTimeSeries";
import {
  aggregateGroupHome,
  type GroupPosition,
} from "@/lib/groupHomeAggregates";
import { formatCurrency } from "@/lib/format";

type LiabilityCategory = "personal" | "institutional";

const LIABILITY_CATEGORIES: LiabilityCategory[] = ["personal", "institutional"];

// Distinct hues, kept clear of the asset palette. Personal = violet,
// institutional = rose.
const CATEGORY_FILLS: Record<LiabilityCategory, string> = {
  personal: "#8b5cf6", // violet-500
  institutional: "#f43f5e", // rose-500
};

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
        latestValue: it.latest_snapshot
          ? Number(it.latest_snapshot.amount)
          : null,
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

  const stackCategories: GroupStackCategory[] = LIABILITY_CATEGORIES.map(
    (c) => ({
      key: c,
      label: t(`liabilities:home.categoryLabel.${c}`),
      color: CATEGORY_FILLS[c],
    }),
  );

  return (
    <div className="space-y-6" data-testid="liabilities-home">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">
          {t("common:home.liabilities.title")}
        </h1>
        <p className="text-sm text-muted-foreground">
          {t("liabilities:home.subtitle")}
        </p>
      </div>

      {liabilities.isPending && (
        <p className="text-sm text-muted-foreground">{t("common:loading")}</p>
      )}

      {liabilities.error && (
        <p className="text-sm text-destructive">
          {t("errors:failedToLoad", {
            message: (liabilities.error as Error).message,
          })}
        </p>
      )}

      <TotalOwedCard
        aggregates={aggregates.byCurrency}
        count={aggregates.count}
      />

      {currencies.map((currency) => (
        <div key={currency} className="space-y-4">
          <ValueCard
            currency={currency}
            series={aggregates.timeSeriesByCurrency.get(currency) ?? []}
          />
          <CategoryStackCard
            currency={currency}
            series={aggregates.categorySeriesByCurrency.get(currency) ?? []}
            categories={stackCategories}
          />
          <CategoryPieCard
            currency={currency}
            slices={buildCategorySlices(
              aggregates.categoryPieByCurrency.get(currency) ?? [],
              t,
            )}
          />
        </div>
      ))}
    </div>
  );
}

type TFn = (key: string, opts?: Record<string, unknown>) => string;

function buildCategorySlices(
  pie: { category: string; value: number }[],
  t: TFn,
): PieSlice[] {
  return LIABILITY_CATEGORIES.map((c) => {
    const found = pie.find((p) => p.category === c);
    return {
      key: c,
      label: t(`liabilities:home.categoryLabel.${c}`),
      value: found?.value ?? 0,
      color: CATEGORY_FILLS[c],
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
  return (
    <div className="rounded-lg border p-4" data-testid="home-total">
      <div className="text-sm text-muted-foreground">
        {t("home.totalOwedTitle")}
      </div>
      <div className="mt-0.5 text-2xl font-semibold tabular-nums">
        {aggregates.map((a, i) => (
          <span key={a.currency}>
            {i > 0 && <span className="mx-2 text-muted-foreground">·</span>}
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
  if (series.length < 2) return null;
  return (
    <Card data-testid={`home-category-stack-${currency}`}>
      <CardHeader>
        <CardTitle>{t("home.categoryStackTitle")}</CardTitle>
        <CardDescription>
          {t("home.categoryStackDescription", { currency })}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <GroupCategoryStackChart
          series={series}
          categories={categories}
          currency={currency}
        />
      </CardContent>
    </Card>
  );
}

function CategoryPieCard({
  currency,
  slices,
}: {
  currency: string;
  slices: PieSlice[];
}) {
  const { t } = useTranslation("liabilities");
  if (slices.every((s) => s.value <= 0)) return null;
  return (
    <Card data-testid={`home-category-pie-${currency}`}>
      <CardHeader>
        <CardTitle>{t("home.categoryPieTitle")}</CardTitle>
        <CardDescription>
          {t("home.categoryPieDescription", { currency })}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <InvestmentPieChart slices={slices} currency={currency} />
      </CardContent>
    </Card>
  );
}
