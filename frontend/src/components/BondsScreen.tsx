import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Table, TableBody, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { SortableHeader } from '@/components/SortableHeader'
import { InvestmentListHeadline } from '@/components/InvestmentListHeadline'
import { ListTimeGraph } from '@/components/ListTimeGraph'
import { ShowInactiveToggle } from '@/components/ShowInactiveToggle'
import {
  RiskProfileFilter,
  type RiskProfileFilterValue,
} from '@/components/RiskProfileFilter'
import { useBonds, useImportCreateBond } from '@/hooks/useInvestments'
import { useInvestmentTimeSeries } from '@/hooks/useInvestmentTimeSeries'
import { useTableSort, type ColumnSort } from '@/hooks/useTableSort'
import { CreateBondDialog } from '@/components/CreateBondDialog'
import { ImportPositionDialog } from '@/components/ImportPositionDialog'
import { BondListRow } from '@/components/BondListRow'
import { isActiveStatus, statusLabel } from '@/lib/lifecycle'
import { aggregateListPositions, type Position } from '@/lib/listAggregates'
import { byNumberNullsLast, byText } from '@/lib/sort'
import type { BondListItem } from '@/api/types'

type Props = {
  onSelect: (id: string) => void
}

type SortKey = 'name' | 'status' | 'value'

type Row = {
  item: BondListItem
  name: string
  status: string
  statusText: string
  amount: number | null
}

const tiebreakByName = (a: Row, b: Row) => a.name.localeCompare(b.name)

export function BondsScreen({ onSelect }: Props) {
  const { t } = useTranslation(['investments', 'common', 'errors'])
  const { data, isPending, error } = useBonds()
  const importMutation = useImportCreateBond()
  const [showInactive, setShowInactive] = useState(false)
  const [riskFilter, setRiskFilter] = useState<RiskProfileFilterValue>('all')

  const noun = t('investments:list.noun')
  const nounPlural = t('investments:list.nounPlural')

  const rows = useMemo<Row[]>(
    () =>
      (data ?? []).map((item) => ({
        item,
        name: item.investment.display_name,
        status: item.investment.status,
        statusText: statusLabel('investments', item.investment.status),
        amount: item.latest_snapshot ? Number(item.latest_snapshot.amount) : null,
      })),
    [data],
  )

  const columns = useMemo<Record<SortKey, ColumnSort<Row>>>(
    () => ({
      name: { dir: 'asc', cmp: byText((r) => r.name) },
      status: { dir: 'asc', cmp: byText((r) => r.statusText) },
      value: { dir: 'desc', cmp: byNumberNullsLast((r) => r.amount) },
    }),
    [],
  )

  const { sorted, sortKey, sortDir, toggle } = useTableSort(rows, columns, {
    defaultKey: 'name',
    tiebreak: tiebreakByName,
  })

  // Headline cost from the list payload (#18); the time-graph value + cost
  // series come from one household-scoped fetch (#22) — the backend replays the
  // bond ledger (every bond carries a Buy now, #27), so no client replay here.
  const timeSeries = useInvestmentTimeSeries()
  const positions = useMemo<Position[]>(
    () =>
      (data ?? []).map((item) => {
        const ts = timeSeries.byId.get(item.investment.id)
        return {
          id: item.investment.id,
          currency: item.investment.native_currency,
          status: item.investment.status,
          terminated_at: item.investment.terminated_at,
          latestValue: item.latest_snapshot
            ? Number(item.latest_snapshot.amount)
            : null,
          cost: Number(item.cost_basis),
          snapshots: ts?.snapshots ?? [],
          costSeries: ts?.costSeries ?? [],
        }
      }),
    [data, timeSeries.byId],
  )
  const aggregates = useMemo(
    () => aggregateListPositions(positions),
    [positions],
  )

  const terminatedCount = rows.filter((r) => !isActiveStatus(r.status)).length
  const visibleRows = (showInactive
    ? sorted
    : sorted.filter((r) => isActiveStatus(r.status))
  ).filter((r) =>
    riskFilter === 'all' ? true : r.item.investment.risk_profile === riskFilter,
  )

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">
            {t('investments:bond.listTitle')}
          </h1>
          <p className="text-sm text-muted-foreground">
            {t('investments:bond.listSubtitle')}
          </p>
        </div>
        <div className="flex items-center gap-2">
          <ImportPositionDialog noun={noun} mutation={importMutation} />
          <CreateBondDialog />
        </div>
      </div>

      <InvestmentListHeadline
        aggregates={aggregates.byCurrency}
        count={aggregates.count}
        noun={noun}
        nounPlural={nounPlural}
        testId="bonds-total"
      />

      <ListTimeGraph timeSeriesByCurrency={aggregates.timeSeriesByCurrency} />

      {isPending && (
        <p className="text-sm text-muted-foreground">{t('common:loading')}</p>
      )}

      {error && (
        <p className="text-sm text-destructive">
          {t('errors:failedToLoad', { message: (error as Error).message })}
        </p>
      )}

      {data && data.length === 0 && (
        <Card>
          <CardHeader>
            <CardTitle>{t('investments:bond.emptyTitle')}</CardTitle>
            <CardDescription>
              {t('investments:bond.emptyBody')}
            </CardDescription>
          </CardHeader>
          <CardContent>
            <CreateBondDialog />
          </CardContent>
        </Card>
      )}

      {data && data.length > 0 && (
        <div className="space-y-3">
          <RiskProfileFilter value={riskFilter} onChange={setRiskFilter} />
          {terminatedCount > 0 && (
            <ShowInactiveToggle
              count={terminatedCount}
              nounPlural={nounPlural}
              checked={showInactive}
              onChange={setShowInactive}
            />
          )}

          {visibleRows.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              {t('common:list.noActive', {
                count: terminatedCount,
                noun,
                nounPlural,
              })}
            </p>
          ) : (
            <Card>
              <CardContent className="p-0">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <SortableHeader
                        label={t('common:tableHeaders.name')}
                        testId="sort-name"
                        active={sortKey === 'name'}
                        dir={sortDir}
                        onSort={() => toggle('name')}
                      />
                      <TableHead>
                        {t('investments:bond.identityHeader')}
                      </TableHead>
                      <SortableHeader
                        label={t('common:tableHeaders.status')}
                        testId="sort-status"
                        active={sortKey === 'status'}
                        dir={sortDir}
                        onSort={() => toggle('status')}
                      />
                      <SortableHeader
                        label={t('investments:bond.sortLatestValue')}
                        testId="sort-value"
                        align="right"
                        active={sortKey === 'value'}
                        dir={sortDir}
                        onSort={() => toggle('value')}
                      />
                      <TableHead className="w-12"></TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {visibleRows.map((r) => (
                      <BondListRow
                        key={r.item.investment.id}
                        item={r.item}
                        onSelect={onSelect}
                      />
                    ))}
                  </TableBody>
                </Table>
              </CardContent>
            </Card>
          )}
        </div>
      )}
    </div>
  )
}
