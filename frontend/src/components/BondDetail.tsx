import { useState } from 'react'
import { Download, Pencil, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Table,
  TableBody,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { PaginationControls } from '@/components/PaginationControls'
import { useBond, useDeleteBond } from '@/hooks/useInvestments'
import {
  useInvestmentSnapshots,
  useCreateInvestmentSnapshot,
  useUpdateInvestmentSnapshot,
  useDeleteInvestmentSnapshot,
  useImportInvestmentSnapshots,
  investmentImportTemplateUrl,
  bondExportUrl,
} from '@/hooks/useInvestmentSnapshots'
import {
  useInvestmentTransactions,
  useCreateInvestmentTransaction,
  useUpdateInvestmentTransaction,
  useDeleteInvestmentTransaction,
} from '@/hooks/useInvestmentTransactions'
import { CreateAccruedInterestSnapshotDialog } from '@/components/CreateAccruedInterestSnapshotDialog'
import { ImportSnapshotsDialog } from '@/components/ImportSnapshotsDialog'
import { CreateTradeTransactionDialog } from '@/components/CreateTradeTransactionDialog'
import { CreateCashIncomeTransactionDialog } from '@/components/CreateCashIncomeTransactionDialog'
import { CreateFeeTransactionDialog } from '@/components/CreateFeeTransactionDialog'
import { CreateMaturityTransactionDialog } from '@/components/CreateMaturityTransactionDialog'
import { TransactionRow } from '@/components/TransactionRow'
import { TerminatePositionDialog } from '@/components/TerminatePositionDialog'
import { StatusBadge } from '@/components/StatusBadge'
import { isActiveStatus } from '@/lib/lifecycle'
import { EditBondDialog } from '@/components/EditBondDialog'
import { ConfirmDialog } from '@/components/ConfirmDialog'
import { AccruedInterestSnapshotRow } from '@/components/AccruedInterestSnapshotRow'
import { SnapshotChart } from '@/components/SnapshotChart'
import { useHouseholdMembers } from '@/hooks/useHouseholdMembers'
import { useSession } from '@/hooks/useSession'
import { formatCurrency, formatDate } from '@/lib/format'
import { ownershipLabel } from '@/lib/ownership'
import { matchesTxnSearch } from '@/lib/transactionSearch'
import { computeCostBasis, costBasisSeries } from '@/lib/costBasis'
import { InvestmentHeadline } from '@/components/InvestmentHeadline'
import { HelpTourButton, type TourStep } from '@/components/HelpTourButton'
import { DetailTagControl } from '@/components/DetailTagControl'

type Props = {
  investmentId: string
  onBack: () => void
}

const PAGE_SIZE = 12

export function BondDetail({ investmentId, onBack }: Props) {
  const { t } = useTranslation(['investments', 'common', 'errors'])
  const { data: bond, isPending, error } = useBond(investmentId)
  const { data: snapshots } = useInvestmentSnapshots(investmentId)
  const { data: transactions } = useInvestmentTransactions(investmentId)
  const deleteMutation = useDeleteBond()
  const createSnapshotMutation = useCreateInvestmentSnapshot(
    investmentId,
    'bonds',
  )
  const updateSnapshotMutation = useUpdateInvestmentSnapshot(
    investmentId,
    'bonds',
  )
  const deleteSnapshotMutation = useDeleteInvestmentSnapshot(
    investmentId,
    'bonds',
  )
  const importSnapshotMutation = useImportInvestmentSnapshots(
    investmentId,
    'bonds',
  )
  const createTransactionMutation = useCreateInvestmentTransaction(
    investmentId,
    'bonds',
  )
  const updateTransactionMutation = useUpdateInvestmentTransaction(investmentId)
  const deleteTransactionMutation = useDeleteInvestmentTransaction(investmentId)
  const { data: members } = useHouseholdMembers()
  const { data: currentUser } = useSession()

  const [editOpen, setEditOpen] = useState(false)
  const [deleteOpen, setDeleteOpen] = useState(false)
  const [page, setPage] = useState(1)
  const [txnPage, setTxnPage] = useState(1)
  const [txnSearch, setTxnSearch] = useState('')

  const totalPages = Math.max(
    1,
    Math.ceil((snapshots?.length ?? 0) / PAGE_SIZE),
  )
  const effectivePage = Math.min(page, totalPages)
  const filteredTransactions = (transactions ?? []).filter((tx) =>
    matchesTxnSearch(tx, txnSearch),
  )
  const totalTxnPages = Math.max(
    1,
    Math.ceil(filteredTransactions.length / PAGE_SIZE),
  )
  const effectiveTxnPage = Math.min(txnPage, totalTxnPages)
  const quantityUnit = t('investments:bond.quantityUnit')

  function handleConfirmDelete() {
    deleteMutation.mutate(investmentId, {
      onSuccess: () => {
        setDeleteOpen(false)
        onBack()
      },
    })
  }

  if (isPending) {
    return <p className="text-sm text-muted-foreground">{t('common:loading')}</p>
  }
  if (error) {
    return (
      <p className="text-sm text-destructive">
        {t('errors:failedToLoad', { message: (error as Error).message })}
      </p>
    )
  }
  if (!bond) return null

  const pageSnapshots = (snapshots ?? []).slice(
    (effectivePage - 1) * PAGE_SIZE,
    effectivePage * PAGE_SIZE,
  )
  const pageTransactions = filteredTransactions.slice(
    (effectiveTxnPage - 1) * PAGE_SIZE,
    effectiveTxnPage * PAGE_SIZE,
  )
  // Lifetime sums of coupon income and trading fees — quick "yield-to-date"
  // glance the user asked for in #14. Maturity payouts intentionally excluded:
  // they're terminal, not recurring income.
  const txnSum = (kind: 'coupon' | 'fee'): number =>
    (transactions ?? []).reduce(
      (acc, tx) =>
        tx.transaction_type === kind ? acc + Number(tx.amount) : acc,
      0,
    )
  const totalCoupons = txnSum('coupon')
  const totalFees = txnSum('fee')
  // Every bond now carries a Buy at placement (issue #27) — govt_primary seeds
  // one at par, secondary-market records the real one — so cost always replays
  // from the ledger; no face_value fallback remains.
  const bondCostSeries = costBasisSeries(snapshots ?? [], transactions ?? [])
  const bondTotalCost = computeCostBasis(transactions ?? []).cost
  const bondLatestSnapshot =
    snapshots && snapshots.length > 0 ? snapshots[0] : null
  // Maturity is uniquely terminal: posting it flips the position to 'matured'
  // (backend hard guard, ADR-0009), after which the transaction-create row is
  // gated off entirely by isActiveStatus below.
  const couponPct = Number(bond.details.coupon_rate).toFixed(2)
  const bondTypeLabel = t(
    bond.details.bond_type === 'govt_primary'
      ? 'investments:bond.bondType.govt_primary'
      : 'investments:bond.bondType.secondary_market',
  )
  const frequencyLabel = t(
    `investments:bond.couponFrequency.${bond.details.coupon_frequency}`,
  )
  const subtitleBits = [
    bond.details.series_code,
    bondTypeLabel,
    bond.details.issuer,
  ].filter(Boolean)

  // Guided tour (issue #23). Steps run top-to-bottom; HelpTourButton prunes any
  // whose anchor isn't rendered this visit (chart needs ≥2 snapshots; the
  // add-row actions hide on closed positions).
  const tourSteps: TourStep[] = [
    {
      element: '[data-testid="tour-overview"]',
      title: t('investments:bond.tour.overviewTitle'),
      description: t('investments:bond.tour.overviewBody'),
    },
    {
      element: '[data-testid="investment-headline"]',
      title: t('investments:bond.tour.headlineTitle'),
      description: t('investments:bond.tour.headlineBody'),
    },
    {
      element: '[data-testid="tour-actions"]',
      title: t('investments:bond.tour.actionsTitle'),
      description: t('investments:bond.tour.actionsBody'),
    },
    {
      element: '[data-testid="tour-details"]',
      title: t('investments:bond.tour.detailsTitle'),
      description: t('investments:bond.tour.detailsBody'),
    },
    {
      element: '[data-testid="tour-chart"]',
      title: t('investments:bond.tour.chartTitle'),
      description: t('investments:bond.tour.chartBody'),
    },
    {
      element: '[data-testid="tour-snapshots"]',
      title: t('investments:bond.tour.snapshotsTitle'),
      description: t('investments:bond.tour.snapshotsBody'),
    },
    {
      element: '[data-testid="tour-transactions"]',
      title: t('investments:bond.tour.transactionsTitle'),
      description: t('investments:bond.tour.transactionsBody'),
    },
  ]

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between gap-4">
        <div>
          <Button
            variant="ghost"
            size="sm"
            onClick={onBack}
            className="-ml-2 mb-1"
          >
            {t('common:actions.back')}
          </Button>
          <h1
            data-testid="tour-overview"
            className="text-2xl font-semibold tracking-tight"
          >
            {bond.investment.display_name}
          </h1>
          <p className="text-sm text-muted-foreground">
            {subtitleBits.join(' · ')}
          </p>
          <InvestmentHeadline
            currency={bond.investment.native_currency}
            latestValue={
              bondLatestSnapshot ? Number(bondLatestSnapshot.amount) : null
            }
            totalCost={bondTotalCost}
            status={bond.investment.status}
            terminatedAt={bond.investment.terminated_at}
          />
          <DetailTagControl group="investment" positionId={bond.investment.id} currentTagId={bond.investment.tag_id} />
        </div>
        <div data-testid="tour-actions" className="flex gap-2">
          <HelpTourButton steps={tourSteps} />
          <Button variant="outline" size="sm" onClick={() => setEditOpen(true)}>
            <Pencil className="mr-1 size-4" />
            {t('common:actions.edit')}
          </Button>
          <TerminatePositionDialog
            group="investments"
            id={bond.investment.id}
            listKey="bonds"
            currentStatus={bond.investment.status}
            currentTerminatedAt={bond.investment.terminated_at}
            currentNote={bond.investment.termination_note}
          />
          <Button
            variant="outline"
            size="sm"
            onClick={() => setDeleteOpen(true)}
          >
            <Trash2 className="mr-1 size-4" />
            {t('common:delete')}
          </Button>
        </div>
      </div>

      <Card data-testid="tour-details">
        <CardHeader>
          <CardTitle>{t('investments:bond.detailsCardTitle')}</CardTitle>
          <CardDescription>
            {t('investments:bond.detailsCardLine', {
              ownership: ownershipLabel(
                bond.investment.ownership_type,
                bond.investment.sole_owner_user_id,
                members,
                currentUser,
              ),
              currency: bond.investment.native_currency,
            })}{' '}
            <StatusBadge group="investments" status={bond.investment.status} />
          </CardDescription>
        </CardHeader>
        <CardContent className="text-sm space-y-1">
          <p>
            <span className="text-muted-foreground">
              {t('investments:bond.faceValueLabel')}
            </span>{' '}
            {formatCurrency(
              bond.outstanding_face,
              bond.investment.native_currency,
            )}
          </p>
          <p>
            <span className="text-muted-foreground">
              {t('investments:bond.couponLabel')}
            </span>{' '}
            {t('investments:bond.couponValue', {
              rate: couponPct,
              frequency: frequencyLabel,
            })}
          </p>
          <p>
            <span className="text-muted-foreground">
              {t('investments:bond.maturityLabel')}
            </span>{' '}
            {formatDate(bond.details.maturity_date)}
          </p>
          {bond.investment.description && (
            <p className="pt-1">{bond.investment.description}</p>
          )}
        </CardContent>
      </Card>

      {snapshots && snapshots.length >= 2 && (
        <Card data-testid="tour-chart">
          <CardHeader>
            <CardTitle>{t('investments:snapshotsCard.chartTitle')}</CardTitle>
            <CardDescription>
              {t('investments:snapshotsCard.chartDescriptionTotal', {
                currency: bond.investment.native_currency,
              })}
            </CardDescription>
          </CardHeader>
          <CardContent>
            <SnapshotChart
              snapshots={snapshots}
              currency={bond.investment.native_currency}
              costSeries={bondCostSeries}
              status={bond.investment.status}
            />
          </CardContent>
        </Card>
      )}

      <Card data-testid="tour-snapshots">
        <CardHeader>
          <div className="flex items-center justify-between gap-4">
            <div>
              <CardTitle>{t('investments:snapshotsCard.title')}</CardTitle>
              <CardDescription>
                {t('investments:bond.snapshotsDescription')}
              </CardDescription>
            </div>
            <div className="flex flex-wrap gap-2">
              {/* Full position workbook (Detail + Snapshots + Transactions);
                  available regardless of status. */}
              <Button asChild size="sm" variant="outline" data-testid="bond-export">
                <a href={bondExportUrl(bond.investment.id)}>
                  <Download className="mr-1 size-4" />
                  {t('common:export.trigger')}
                </a>
              </Button>
              {isActiveStatus(bond.investment.status) && (
                <>
                  <CreateAccruedInterestSnapshotDialog
                    currency={bond.investment.native_currency}
                    mutation={createSnapshotMutation}
                    carryover={
                      bondLatestSnapshot
                        ? {
                            amount: bondLatestSnapshot.amount,
                            accrued_interest:
                              bondLatestSnapshot.accrued_interest,
                            lastSnapshotMonth: bondLatestSnapshot.year_month,
                          }
                        : null
                    }
                  />
                  <ImportSnapshotsDialog
                    templateUrl={investmentImportTemplateUrl(bond.investment.id)}
                    mutation={importSnapshotMutation}
                    currency={bond.investment.native_currency}
                  />
                </>
              )}
            </div>
          </div>
        </CardHeader>
        <CardContent className="p-0">
          {!snapshots || snapshots.length === 0 ? (
            <p className="p-6 text-sm text-muted-foreground">
              {t('investments:bond.snapshotsEmpty')}
            </p>
          ) : (
            <>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('investments:snapshotsCard.monthHeader')}</TableHead>
                    <TableHead className="text-right">{t('investments:snapshotsCard.principalHeader')}</TableHead>
                    <TableHead className="text-right">{t('investments:snapshotsCard.accruedHeader')}</TableHead>
                    <TableHead className="text-right">{t('investments:snapshotsCard.totalValueHeader')}</TableHead>
                    <TableHead>{t('investments:snapshotsCard.notesHeader')}</TableHead>
                    <TableHead className="w-12"></TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {pageSnapshots.map((s) => (
                    <AccruedInterestSnapshotRow
                      key={s.id}
                      snapshot={s}
                      updateMutation={updateSnapshotMutation}
                      deleteMutation={deleteSnapshotMutation}
                    />
                  ))}
                </TableBody>
              </Table>
              {totalPages > 1 && (
                <div className="px-6 py-3 border-t">
                  <PaginationControls
                    page={effectivePage}
                    totalPages={totalPages}
                    onPageChange={setPage}
                  />
                </div>
              )}
            </>
          )}
        </CardContent>
      </Card>

      <Card data-testid="tour-transactions">
        <CardHeader>
          <div className="flex items-center justify-between gap-4">
            <div>
              <CardTitle>{t('investments:transactions.cardTitle')}</CardTitle>
              <CardDescription>
                {t('investments:bond.transactionsDescription')}
              </CardDescription>
            </div>
            {isActiveStatus(bond.investment.status) && (
              <div className="flex flex-wrap gap-2">
                <CreateTradeTransactionDialog
                  currency={bond.investment.native_currency}
                  txnType="buy"
                  quantityUnit={quantityUnit}
                  mutation={createTransactionMutation}
                />
                <CreateTradeTransactionDialog
                  currency={bond.investment.native_currency}
                  txnType="sell"
                  quantityUnit={quantityUnit}
                  mutation={createTransactionMutation}
                />
                <CreateCashIncomeTransactionDialog
                  currency={bond.investment.native_currency}
                  txnType="coupon"
                  mutation={createTransactionMutation}
                />
                <CreateFeeTransactionDialog
                  currency={bond.investment.native_currency}
                  quantityUnit={quantityUnit}
                  mutation={createTransactionMutation}
                />
                <CreateMaturityTransactionDialog
                  currency={bond.investment.native_currency}
                  mutation={createTransactionMutation}
                />
              </div>
            )}
          </div>
        </CardHeader>
        <CardContent className="p-0">
          {!transactions || transactions.length === 0 ? (
            <p className="p-6 text-sm text-muted-foreground">
              {t('investments:bond.transactionsEmpty')}
            </p>
          ) : (
            <>
              <div className="flex flex-wrap items-center justify-between gap-4 border-b px-6 py-3">
                <div className="flex flex-wrap gap-x-6 gap-y-1 text-sm">
                  <div>
                    <span className="text-muted-foreground">
                      {t('investments:bond.totalCouponsLabel')}
                    </span>{' '}
                    <span className="tabular-nums">
                      {formatCurrency(
                        totalCoupons.toString(),
                        bond.investment.native_currency,
                      )}
                    </span>
                  </div>
                  <div>
                    <span className="text-muted-foreground">
                      {t('investments:bond.totalFeesLabel')}
                    </span>{' '}
                    <span className="tabular-nums">
                      {formatCurrency(
                        totalFees.toString(),
                        bond.investment.native_currency,
                      )}
                    </span>
                  </div>
                </div>
                <Input
                  data-testid="txn-search"
                  placeholder={t('investments:transactions.searchPlaceholder')}
                  value={txnSearch}
                  onChange={(e) => setTxnSearch(e.target.value)}
                  className="max-w-xs"
                />
              </div>
              {filteredTransactions.length === 0 ? (
                <p className="p-6 text-sm text-muted-foreground">
                  {t('investments:transactions.searchEmpty')}
                </p>
              ) : (
                <>
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>{t('investments:transactions.dateHeader')}</TableHead>
                        <TableHead>{t('investments:transactions.typeHeader')}</TableHead>
                        <TableHead className="text-right">{t('investments:transactions.cashImpactHeader')}</TableHead>
                        <TableHead>{t('investments:transactions.notesHeader')}</TableHead>
                        <TableHead className="w-12"></TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {pageTransactions.map((tx) => (
                        <TransactionRow
                          key={tx.id}
                          transaction={tx}
                          quantityUnit={quantityUnit}
                          updateMutation={updateTransactionMutation}
                          deleteMutation={deleteTransactionMutation}
                        />
                      ))}
                    </TableBody>
                  </Table>
                  {totalTxnPages > 1 && (
                    <div className="px-6 py-3 border-t">
                      <PaginationControls
                        page={effectiveTxnPage}
                        totalPages={totalTxnPages}
                        onPageChange={setTxnPage}
                      />
                    </div>
                  )}
                </>
              )}
            </>
          )}
        </CardContent>
      </Card>

      <EditBondDialog
        key={bond.investment.id}
        open={editOpen}
        onOpenChange={setEditOpen}
        bond={bond}
      />
      <ConfirmDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        title={t('investments:bond.deleteTitle')}
        description={t('investments:bond.deleteDetailDescription')}
        confirmLabel={t('common:delete')}
        destructive
        pending={deleteMutation.isPending}
        onConfirm={handleConfirmDelete}
      />
    </div>
  )
}
