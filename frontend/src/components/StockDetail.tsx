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
import { useStock, useDeleteStock } from '@/hooks/useInvestments'
import {
  useInvestmentSnapshots,
  useCreateInvestmentSnapshot,
  useUpdateInvestmentSnapshot,
  useDeleteInvestmentSnapshot,
  useImportInvestmentSnapshots,
  investmentImportTemplateUrl,
  stockExportUrl,
} from '@/hooks/useInvestmentSnapshots'
import {
  useInvestmentTransactions,
  useCreateInvestmentTransaction,
  useUpdateInvestmentTransaction,
  useDeleteInvestmentTransaction,
} from '@/hooks/useInvestmentTransactions'
import { CreateQuantityPriceSnapshotDialog } from '@/components/CreateQuantityPriceSnapshotDialog'
import { ImportSnapshotsDialog } from '@/components/ImportSnapshotsDialog'
import { CreateTradeTransactionDialog } from '@/components/CreateTradeTransactionDialog'
import { CreateCashIncomeTransactionDialog } from '@/components/CreateCashIncomeTransactionDialog'
import { CreateFeeTransactionDialog } from '@/components/CreateFeeTransactionDialog'
import { TransactionRow } from '@/components/TransactionRow'
import { TerminatePositionDialog } from '@/components/TerminatePositionDialog'
import { StatusBadge } from '@/components/StatusBadge'
import { isActiveStatus } from '@/lib/lifecycle'
import { EditStockDialog } from '@/components/EditStockDialog'
import { ConfirmDialog } from '@/components/ConfirmDialog'
import { QuantityPriceSnapshotRow } from '@/components/QuantityPriceSnapshotRow'
import { SnapshotChart } from '@/components/SnapshotChart'
import { HelpTourButton, type TourStep } from '@/components/HelpTourButton'
import { DetailTagControl } from '@/components/DetailTagControl'
import { useHouseholdMembers } from '@/hooks/useHouseholdMembers'
import { useSession } from '@/hooks/useSession'
import { ownershipLabel } from '@/lib/ownership'
import { reconcileQuantity } from '@/lib/reconciliation'
import { matchesTxnSearch } from '@/lib/transactionSearch'
import { computeCostBasis, costBasisSeries } from '@/lib/costBasis'
import { InvestmentHeadline } from '@/components/InvestmentHeadline'

type Props = {
  investmentId: string
  onBack: () => void
}

const PAGE_SIZE = 12

export function StockDetail({ investmentId, onBack }: Props) {
  const { t } = useTranslation(['investments', 'common', 'errors'])
  const { data: stock, isPending, error } = useStock(investmentId)
  const { data: snapshots } = useInvestmentSnapshots(investmentId)
  const { data: transactions } = useInvestmentTransactions(investmentId)
  const deleteMutation = useDeleteStock()
  const createSnapshotMutation = useCreateInvestmentSnapshot(
    investmentId,
    'stocks',
  )
  const updateSnapshotMutation = useUpdateInvestmentSnapshot(
    investmentId,
    'stocks',
  )
  const deleteSnapshotMutation = useDeleteInvestmentSnapshot(
    investmentId,
    'stocks',
  )
  const importSnapshotMutation = useImportInvestmentSnapshots(
    investmentId,
    'stocks',
  )
  const createTransactionMutation = useCreateInvestmentTransaction(investmentId)
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
  const latestSnapshot = snapshots && snapshots.length > 0 ? snapshots[0] : null
  const recon = reconcileQuantity(latestSnapshot, transactions)
  const quantityUnit = t('investments:stock.quantityUnit')

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
  if (!stock) return null

  const pageSnapshots = (snapshots ?? []).slice(
    (effectivePage - 1) * PAGE_SIZE,
    effectivePage * PAGE_SIZE,
  )
  const pageTransactions = filteredTransactions.slice(
    (effectiveTxnPage - 1) * PAGE_SIZE,
    effectiveTxnPage * PAGE_SIZE,
  )

  const tourSteps: TourStep[] = [
    {
      element: '[data-testid="tour-overview"]',
      title: t('investments:stock.tour.overviewTitle'),
      description: t('investments:stock.tour.overviewBody'),
    },
    {
      element: '[data-testid="investment-headline"]',
      title: t('investments:stock.tour.headlineTitle'),
      description: t('investments:stock.tour.headlineBody'),
    },
    {
      element: '[data-testid="tour-actions"]',
      title: t('investments:stock.tour.actionsTitle'),
      description: t('investments:stock.tour.actionsBody'),
    },
    {
      element: '[data-testid="tour-details"]',
      title: t('investments:stock.tour.detailsTitle'),
      description: t('investments:stock.tour.detailsBody'),
    },
    {
      element: '[data-testid="tour-chart"]',
      title: t('investments:stock.tour.chartTitle'),
      description: t('investments:stock.tour.chartBody'),
    },
    {
      element: '[data-testid="tour-snapshots"]',
      title: t('investments:stock.tour.snapshotsTitle'),
      description: t('investments:stock.tour.snapshotsBody'),
    },
    {
      element: '[data-testid="tour-transactions"]',
      title: t('investments:stock.tour.transactionsTitle'),
      description: t('investments:stock.tour.transactionsBody'),
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
          <h1 data-testid="tour-overview" className="text-2xl font-semibold tracking-tight">
            {stock.investment.display_name}
          </h1>
          <p className="text-sm text-muted-foreground">
            {stock.details.ticker} · {stock.details.exchange}
          </p>
          <InvestmentHeadline
            currency={stock.investment.native_currency}
            latestValue={latestSnapshot ? Number(latestSnapshot.amount) : null}
            totalCost={computeCostBasis(transactions ?? []).cost}
            status={stock.investment.status}
            terminatedAt={stock.investment.terminated_at}
          />
          <DetailTagControl group="investment" positionId={stock.investment.id} currentTagId={stock.investment.tag_id} />
        </div>
        <div data-testid="tour-actions" className="flex gap-2">
          <HelpTourButton steps={tourSteps} />
          <Button variant="outline" size="sm" onClick={() => setEditOpen(true)}>
            <Pencil className="mr-1 size-4" />
            {t('common:actions.edit')}
          </Button>
          <TerminatePositionDialog
            group="investments"
            id={stock.investment.id}
            listKey="stocks"
            currentStatus={stock.investment.status}
            currentTerminatedAt={stock.investment.terminated_at}
            currentNote={stock.investment.termination_note}
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
          <CardTitle>{t('investments:stock.detailsCardTitle')}</CardTitle>
          <CardDescription>
            {t('investments:stock.detailsCardLine', {
              ownership: ownershipLabel(
                stock.investment.ownership_type,
                stock.investment.sole_owner_user_id,
                members,
                currentUser,
              ),
              currency: stock.investment.native_currency,
            })}{' '}
            <StatusBadge group="investments" status={stock.investment.status} />
          </CardDescription>
        </CardHeader>
        {stock.investment.description && (
          <CardContent className="text-sm">
            {stock.investment.description}
          </CardContent>
        )}
      </Card>

      {snapshots && snapshots.length >= 2 && (
        <Card data-testid="tour-chart">
          <CardHeader>
            <CardTitle>{t('investments:snapshotsCard.chartTitle')}</CardTitle>
            <CardDescription>
              {t('investments:snapshotsCard.chartDescription', {
                currency: stock.investment.native_currency,
              })}
            </CardDescription>
          </CardHeader>
          <CardContent>
            <SnapshotChart
              snapshots={snapshots}
              currency={stock.investment.native_currency}
              costSeries={costBasisSeries(snapshots, transactions ?? [])}
              status={stock.investment.status}
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
                {t('investments:stock.snapshotsDescription')}
              </CardDescription>
            </div>
            <div className="flex flex-wrap gap-2">
              {/* Export the full position workbook (Detail + Snapshots +
                  Transactions). Plain anchor download — session cookie rides
                  along same-origin. Available regardless of status so a
                  terminated position can still be backed up. */}
              <Button asChild size="sm" variant="outline" data-testid="stock-export">
                <a href={stockExportUrl(stock.investment.id)}>
                  <Download className="mr-1 size-4" />
                  {t('common:export.trigger')}
                </a>
              </Button>
              {isActiveStatus(stock.investment.status) && (
                <>
                  <CreateQuantityPriceSnapshotDialog
                    currency={stock.investment.native_currency}
                    mutation={createSnapshotMutation}
                    carryover={
                      latestSnapshot
                        ? {
                            quantity: latestSnapshot.quantity,
                            price_per_unit: latestSnapshot.price_per_unit,
                            lastSnapshotMonth: latestSnapshot.year_month,
                          }
                        : null
                    }
                  />
                  <ImportSnapshotsDialog
                    templateUrl={investmentImportTemplateUrl(stock.investment.id)}
                    mutation={importSnapshotMutation}
                    currency={stock.investment.native_currency}
                  />
                </>
              )}
            </div>
          </div>
        </CardHeader>
        <CardContent className="p-0">
          {!snapshots || snapshots.length === 0 ? (
            <p className="p-6 text-sm text-muted-foreground">
              {t('investments:stock.snapshotsEmpty')}
            </p>
          ) : (
            <>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('investments:snapshotsCard.monthHeader')}</TableHead>
                    <TableHead className="text-right">{t('investments:stock.quantityHeader')}</TableHead>
                    <TableHead className="text-right">{t('investments:stock.priceHeader')}</TableHead>
                    <TableHead className="text-right">{t('investments:snapshotsCard.totalValueHeader')}</TableHead>
                    <TableHead>{t('investments:snapshotsCard.notesHeader')}</TableHead>
                    <TableHead className="w-12"></TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {pageSnapshots.map((s) => (
                    <QuantityPriceSnapshotRow
                      key={s.id}
                      snapshot={s}
                      quantityUnit={quantityUnit}
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
                {t('investments:stock.transactionsDescription')}
              </CardDescription>
            </div>
            {isActiveStatus(stock.investment.status) && (
              <div className="flex flex-wrap gap-2">
                <CreateTradeTransactionDialog
                  currency={stock.investment.native_currency}
                  txnType="buy"
                  quantityUnit={quantityUnit}
                  mutation={createTransactionMutation}
                />
                <CreateTradeTransactionDialog
                  currency={stock.investment.native_currency}
                  txnType="sell"
                  quantityUnit={quantityUnit}
                  mutation={createTransactionMutation}
                />
                <CreateCashIncomeTransactionDialog
                  currency={stock.investment.native_currency}
                  txnType="dividend"
                  mutation={createTransactionMutation}
                />
                <CreateFeeTransactionDialog
                  currency={stock.investment.native_currency}
                  quantityUnit={quantityUnit}
                  mutation={createTransactionMutation}
                />
              </div>
            )}
          </div>
        </CardHeader>
        <CardContent className="p-0">
          {recon && !recon.matches && (
            <div className="mx-6 mb-3 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-900">
              {t('investments:stock.reconcileWarning', {
                actual: recon.actual,
                expected: recon.expected,
              })}
            </div>
          )}
          {!transactions || transactions.length === 0 ? (
            <p className="p-6 text-sm text-muted-foreground">
              {t('investments:stock.transactionsEmpty')}
            </p>
          ) : (
            <>
              <div className="border-b px-6 py-3">
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

      <EditStockDialog
        key={stock.investment.id}
        open={editOpen}
        onOpenChange={setEditOpen}
        stock={stock}
      />
      <ConfirmDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        title={t('investments:stock.deleteTitle')}
        description={t('investments:stock.deleteDetailDescription')}
        confirmLabel={t('common:delete')}
        destructive
        pending={deleteMutation.isPending}
        onConfirm={handleConfirmDelete}
      />
    </div>
  )
}
