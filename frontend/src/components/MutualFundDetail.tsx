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
import { useMutualFund, useDeleteMutualFund } from '@/hooks/useInvestments'
import {
  useInvestmentSnapshots,
  useCreateInvestmentSnapshot,
  useUpdateInvestmentSnapshot,
  useDeleteInvestmentSnapshot,
  useImportInvestmentSnapshots,
  investmentImportTemplateUrl,
  mutualFundExportUrl,
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
import { EditMutualFundDialog } from '@/components/EditMutualFundDialog'
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

export function MutualFundDetail({ investmentId, onBack }: Props) {
  const { t } = useTranslation(['investments', 'common', 'errors'])
  const { data: mf, isPending, error } = useMutualFund(investmentId)
  const { data: snapshots } = useInvestmentSnapshots(investmentId)
  const { data: transactions } = useInvestmentTransactions(investmentId)
  const deleteMutation = useDeleteMutualFund()
  const createSnapshotMutation = useCreateInvestmentSnapshot(
    investmentId,
    'mutual-funds',
  )
  const updateSnapshotMutation = useUpdateInvestmentSnapshot(
    investmentId,
    'mutual-funds',
  )
  const deleteSnapshotMutation = useDeleteInvestmentSnapshot(
    investmentId,
    'mutual-funds',
  )
  const importSnapshotMutation = useImportInvestmentSnapshots(
    investmentId,
    'mutual-funds',
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
  const quantityUnit = t('investments:mutualFund.quantityUnit')

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
  if (!mf) return null

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
      title: t('investments:mutualFund.tour.overviewTitle'),
      description: t('investments:mutualFund.tour.overviewBody'),
    },
    {
      element: '[data-testid="investment-headline"]',
      title: t('investments:mutualFund.tour.headlineTitle'),
      description: t('investments:mutualFund.tour.headlineBody'),
    },
    {
      element: '[data-testid="tour-actions"]',
      title: t('investments:mutualFund.tour.actionsTitle'),
      description: t('investments:mutualFund.tour.actionsBody'),
    },
    {
      element: '[data-testid="tour-details"]',
      title: t('investments:mutualFund.tour.detailsTitle'),
      description: t('investments:mutualFund.tour.detailsBody'),
    },
    {
      element: '[data-testid="tour-chart"]',
      title: t('investments:mutualFund.tour.chartTitle'),
      description: t('investments:mutualFund.tour.chartBody'),
    },
    {
      element: '[data-testid="tour-snapshots"]',
      title: t('investments:mutualFund.tour.snapshotsTitle'),
      description: t('investments:mutualFund.tour.snapshotsBody'),
    },
    {
      element: '[data-testid="tour-transactions"]',
      title: t('investments:mutualFund.tour.transactionsTitle'),
      description: t('investments:mutualFund.tour.transactionsBody'),
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
            {mf.investment.display_name}
          </h1>
          <p className="text-sm text-muted-foreground">
            {mf.details.fund_code}
            {mf.details.fund_manager && ` · ${mf.details.fund_manager}`}
            {` · ${t(`investments:mutualFund.fundType.short.${mf.details.fund_type}`)}`}
          </p>
          <InvestmentHeadline
            currency={mf.investment.native_currency}
            latestValue={latestSnapshot ? Number(latestSnapshot.amount) : null}
            totalCost={computeCostBasis(transactions ?? []).cost}
            status={mf.investment.status}
            terminatedAt={mf.investment.terminated_at}
          />
          <DetailTagControl group="investment" positionId={mf.investment.id} currentTagId={mf.investment.tag_id} />
        </div>
        <div data-testid="tour-actions" className="flex gap-2">
          <HelpTourButton steps={tourSteps} />
          <Button variant="outline" size="sm" onClick={() => setEditOpen(true)}>
            <Pencil className="mr-1 size-4" />
            {t('common:actions.edit')}
          </Button>
          <TerminatePositionDialog
            group="investments"
            id={mf.investment.id}
            listKey="mutual-funds"
            currentStatus={mf.investment.status}
            currentTerminatedAt={mf.investment.terminated_at}
            currentNote={mf.investment.termination_note}
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
          <CardTitle>{t('investments:mutualFund.detailsCardTitle')}</CardTitle>
          <CardDescription>
            {t('investments:mutualFund.detailsCardLine', {
              ownership: ownershipLabel(
                mf.investment.ownership_type,
                mf.investment.sole_owner_user_id,
                members,
                currentUser,
              ),
              currency: mf.investment.native_currency,
            })}{' '}
            <StatusBadge group="investments" status={mf.investment.status} />
          </CardDescription>
        </CardHeader>
        {mf.investment.description && (
          <CardContent className="text-sm">
            {mf.investment.description}
          </CardContent>
        )}
      </Card>

      {snapshots && snapshots.length >= 2 && (
        <Card data-testid="tour-chart">
          <CardHeader>
            <CardTitle>{t('investments:snapshotsCard.chartTitle')}</CardTitle>
            <CardDescription>
              {t('investments:snapshotsCard.chartDescription', {
                currency: mf.investment.native_currency,
              })}
            </CardDescription>
          </CardHeader>
          <CardContent>
            <SnapshotChart
              snapshots={snapshots}
              currency={mf.investment.native_currency}
              costSeries={costBasisSeries(snapshots, transactions ?? [])}
              status={mf.investment.status}
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
                {t('investments:mutualFund.snapshotsDescription')}
              </CardDescription>
            </div>
            <div className="flex flex-wrap gap-2">
              {/* Full position workbook (Detail + Snapshots + Transactions);
                  available regardless of status. */}
              <Button asChild size="sm" variant="outline" data-testid="mutual-fund-export">
                <a href={mutualFundExportUrl(mf.investment.id)}>
                  <Download className="mr-1 size-4" />
                  {t('common:export.trigger')}
                </a>
              </Button>
              {isActiveStatus(mf.investment.status) && (
                <>
                  <CreateQuantityPriceSnapshotDialog
                    currency={mf.investment.native_currency}
                    mutation={createSnapshotMutation}
                    carryover={
                      latestSnapshot
                        ? {
                            quantity: latestSnapshot.quantity,
                            price_per_unit: latestSnapshot.price_per_unit,
                          }
                        : null
                    }
                  />
                  <ImportSnapshotsDialog
                    templateUrl={investmentImportTemplateUrl(mf.investment.id)}
                    mutation={importSnapshotMutation}
                    currency={mf.investment.native_currency}
                  />
                </>
              )}
            </div>
          </div>
        </CardHeader>
        <CardContent className="p-0">
          {!snapshots || snapshots.length === 0 ? (
            <p className="p-6 text-sm text-muted-foreground">
              {t('investments:mutualFund.snapshotsEmpty')}
            </p>
          ) : (
            <>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('investments:snapshotsCard.monthHeader')}</TableHead>
                    <TableHead className="text-right">{t('investments:mutualFund.unitsHeader')}</TableHead>
                    <TableHead className="text-right">{t('investments:mutualFund.navHeader')}</TableHead>
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
                {t('investments:mutualFund.transactionsDescription')}
              </CardDescription>
            </div>
            {isActiveStatus(mf.investment.status) && (
              <div className="flex flex-wrap gap-2">
                <CreateTradeTransactionDialog
                  currency={mf.investment.native_currency}
                  txnType="buy"
                  quantityUnit={quantityUnit}
                  mutation={createTransactionMutation}
                />
                <CreateTradeTransactionDialog
                  currency={mf.investment.native_currency}
                  txnType="sell"
                  quantityUnit={quantityUnit}
                  mutation={createTransactionMutation}
                />
                <CreateCashIncomeTransactionDialog
                  currency={mf.investment.native_currency}
                  txnType="distribution"
                  mutation={createTransactionMutation}
                />
                <CreateFeeTransactionDialog
                  currency={mf.investment.native_currency}
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
              {t('investments:mutualFund.reconcileWarning', {
                actual: recon.actual,
                expected: recon.expected,
              })}
            </div>
          )}
          {!transactions || transactions.length === 0 ? (
            <p className="p-6 text-sm text-muted-foreground">
              {t('investments:mutualFund.transactionsEmpty')}
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

      <EditMutualFundDialog
        key={mf.investment.id}
        open={editOpen}
        onOpenChange={setEditOpen}
        mutualFund={mf}
      />
      <ConfirmDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        title={t('investments:mutualFund.deleteTitle')}
        description={t('investments:mutualFund.deleteDetailDescription')}
        confirmLabel={t('common:delete')}
        destructive
        pending={deleteMutation.isPending}
        onConfirm={handleConfirmDelete}
      />
    </div>
  )
}
