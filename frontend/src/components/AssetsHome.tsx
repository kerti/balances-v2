// Assets landing page (epic #204, slice 1) — the value-only parity twin of
// InvestmentsHome.
//
// Aggregates across the three asset subtypes (bank account / property /
// vehicle — ADR-0022 shared snapshot table) into one set of per-currency
// cards:
//   1. Total value headline (no cost basis — assets have no ledger).
//   2. Total value over time (one line per currency).
//   3. 100%-stacked category share over time (one card per currency).
//   4. Current category-mix pie (one per currency).
//
// **No FX.** Mirrors the 14c list-screen convention: each currency renders its
// own card-set. Headline + pie are active-only; the time + stack series
// include terminated positions historically (capped at terminated_at), via
// the shared `aggregateGroupHome` helper.

import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { SnapshotChart } from '@/components/SnapshotChart'
import {
  GroupCategoryStackChart,
  type GroupStackCategory,
} from '@/components/GroupCategoryStackChart'
import {
  InvestmentPieChart,
  type PieSlice,
} from '@/components/InvestmentPieChart'
import { useBankAccounts } from '@/hooks/useBankAccounts'
import { useProperties } from '@/hooks/useProperties'
import { useVehicles } from '@/hooks/useVehicles'
import { useAssetTimeSeries } from '@/hooks/useAssetTimeSeries'
import {
  aggregateGroupHome,
  type GroupPosition,
} from '@/lib/groupHomeAggregates'
import { formatCurrency } from '@/lib/format'
import type {
  Asset,
  AssetSnapshot,
  BankAccountListItem,
  PropertyListItem,
  VehicleListItem,
} from '@/api/types'

type AssetCategory = 'bankAccount' | 'property' | 'vehicle'

const ASSET_CATEGORIES: AssetCategory[] = ['bankAccount', 'property', 'vehicle']

// Distinct Tailwind 500-level hues, same palette family as the investment
// category fills so the app reads consistently. Bank = emerald (cash tone),
// property = blue, vehicle = amber.
const CATEGORY_FILLS: Record<AssetCategory, string> = {
  bankAccount: '#10b981', // emerald-500
  property: '#3b82f6', // blue-500
  vehicle: '#f59e0b', // amber-500
}

// The common subset of every subtype's *ListItem the aggregation reads.
type AssetListItem = {
  asset: Asset
  latest_snapshot: AssetSnapshot | null
}

export function AssetsHome() {
  const { t } = useTranslation(['common', 'assets', 'errors'])
  const bankAccounts = useBankAccounts()
  const properties = useProperties()
  const vehicles = useVehicles()
  const timeSeries = useAssetTimeSeries()

  const positions = useMemo<GroupPosition[]>(() => {
    const out: GroupPosition[] = []
    const push = (
      items: AssetListItem[] | undefined,
      category: AssetCategory,
    ) => {
      for (const it of items ?? []) {
        const ts = timeSeries.byId.get(it.asset.id)
        out.push({
          id: it.asset.id,
          currency: it.asset.native_currency,
          status: it.asset.status,
          terminated_at: it.asset.terminated_at,
          latestValue: it.latest_snapshot
            ? Number(it.latest_snapshot.amount)
            : null,
          snapshots: ts?.snapshots ?? [],
          category,
        })
      }
    }
    push(bankAccounts.data as BankAccountListItem[] | undefined, 'bankAccount')
    push(properties.data as PropertyListItem[] | undefined, 'property')
    push(vehicles.data as VehicleListItem[] | undefined, 'vehicle')
    return out
  }, [bankAccounts.data, properties.data, vehicles.data, timeSeries.byId])

  const aggregates = useMemo(
    () => aggregateGroupHome(positions, ASSET_CATEGORIES),
    [positions],
  )

  const anyPending =
    bankAccounts.isPending || properties.isPending || vehicles.isPending
  const firstError = bankAccounts.error ?? properties.error ?? vehicles.error

  const currencies = aggregates.byCurrency.map((c) => c.currency)

  const stackCategories: GroupStackCategory[] = ASSET_CATEGORIES.map((c) => ({
    key: c,
    label: t(`assets:home.categoryLabel.${c}`),
    color: CATEGORY_FILLS[c],
  }))

  return (
    <div className="space-y-6" data-testid="assets-home">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">
          {t('common:home.assets.title')}
        </h1>
        <p className="text-sm text-muted-foreground">
          {t('assets:home.subtitle')}
        </p>
      </div>

      {anyPending && (
        <p className="text-sm text-muted-foreground">{t('common:loading')}</p>
      )}

      {firstError && (
        <p className="text-sm text-destructive">
          {t('errors:failedToLoad', {
            message: (firstError as Error).message,
          })}
        </p>
      )}

      <TotalValueCard
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
  )
}

type TFn = (key: string, opts?: Record<string, unknown>) => string

function buildCategorySlices(
  pie: { category: string; value: number }[],
  t: TFn,
): PieSlice[] {
  return ASSET_CATEGORIES.map((c) => {
    const found = pie.find((p) => p.category === c)
    return {
      key: c,
      label: t(`assets:home.categoryLabel.${c}`),
      value: found?.value ?? 0,
      color: CATEGORY_FILLS[c],
    }
  })
}

function TotalValueCard({
  aggregates,
  count,
}: {
  aggregates: { currency: string; value: number }[]
  count: number
}) {
  const { t } = useTranslation('assets')
  if (aggregates.length === 0) return null
  return (
    <div className="rounded-lg border p-4" data-testid="home-total">
      <div className="text-sm text-muted-foreground">
        {t('home.totalValueTitle')}
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
        {t('home.totalValueCount', { count })}
      </div>
    </div>
  )
}

function ValueCard({
  currency,
  series,
}: {
  currency: string
  series: { year_month: string; value: number }[]
}) {
  const { t } = useTranslation('assets')
  if (series.length < 2) return null
  return (
    <Card data-testid={`home-value-${currency}`}>
      <CardHeader>
        <CardTitle>{t('home.valueChartTitle')}</CardTitle>
        <CardDescription>
          {t('home.valueChartDescription', { currency })}
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
  )
}

function CategoryStackCard({
  currency,
  series,
  categories,
}: {
  currency: string
  series: Parameters<typeof GroupCategoryStackChart>[0]['series']
  categories: GroupStackCategory[]
}) {
  const { t } = useTranslation('assets')
  if (series.length < 2) return null
  return (
    <Card data-testid={`home-category-stack-${currency}`}>
      <CardHeader>
        <CardTitle>{t('home.categoryStackTitle')}</CardTitle>
        <CardDescription>
          {t('home.categoryStackDescription', { currency })}
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
  )
}

function CategoryPieCard({
  currency,
  slices,
}: {
  currency: string
  slices: PieSlice[]
}) {
  const { t } = useTranslation('assets')
  if (slices.every((s) => s.value <= 0)) return null
  return (
    <Card data-testid={`home-category-pie-${currency}`}>
      <CardHeader>
        <CardTitle>{t('home.categoryPieTitle')}</CardTitle>
        <CardDescription>
          {t('home.categoryPieDescription', { currency })}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <InvestmentPieChart slices={slices} currency={currency} />
      </CardContent>
    </Card>
  )
}
