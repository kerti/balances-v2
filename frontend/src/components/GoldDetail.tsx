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
import { useGold, useDeleteGold } from '@/hooks/useInvestments'
import {
  useInvestmentSnapshots,
  useCreateInvestmentSnapshot,
  useUpdateInvestmentSnapshot,
  useDeleteInvestmentSnapshot,
  useImportInvestmentSnapshots,
  investmentImportTemplateUrl,
  goldExportUrl,
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
import { CreateFeeTransactionDialog } from '@/components/CreateFeeTransactionDialog'
import { TransactionRow } from '@/components/TransactionRow'
import { TerminatePositionDialog } from '@/components/TerminatePositionDialog'
import { StatusBadge } from '@/components/StatusBadge'
import { isActiveStatus } from '@/lib/lifecycle'
import { EditGoldDialog } from '@/components/EditGoldDialog'
import { ConfirmDialog } from '@/components/ConfirmDialog'
import { QuantityPriceSnapshotRow } from '@/components/QuantityPriceSnapshotRow'
import { SnapshotChart } from '@/components/SnapshotChart'
import { HelpTourButton, type TourStep } from '@/components/HelpTourButton'
import { DetailTagControl } from '@/components/DetailTagControl'
import { useHouseholdMembers } from '@/hooks/useHouseholdMembers'
import { useSession } from '@/hooks/useSession'
import { formatGoldPurity } from '@/lib/gold'
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

export function GoldDetail({ investmentId, onBack }: Props) {
  const { t } = useTranslation(['investments', 'common', 'errors'])
  const { data: gold, isPending, error } = useGold(investmentId)
  const { data: snapshots } = useInvestmentSnapshots(investmentId)
  const { data: transactions } = useInvestmentTransactions(investmentId)
  const deleteMutation = useDeleteGold()
  const createSnapshotMutation = useCreateInvestmentSnapshot(
    investmentId,
    'golds',
  )
  const updateSnapshotMutation = useUpdateInvestmentSnapshot(
    investmentId,
    'golds',
  )
  const deleteSnapshotMutation = useDeleteInvestmentSnapshot(
    investmentId,
    'golds',
  )
  const importSnapshotMutation = useImportInvestmentSnapshots(
    investmentId,
    'golds',
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
  const quantityUnit = t('investments:gold.quantityUnit')

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
  if (!gold) return null

  const pageSnapshots = (snapshots ?? []).slice(
    (effectivePage - 1) * PAGE_SIZE,
    effectivePage * PAGE_SIZE,
  )
  const pageTransactions = filteredTransactions.slice(
    (effectiveTxnPage - 1) * PAGE_SIZE,
    effectiveTxnPage * PAGE_SIZE,
  )
  const formLabel = t(`investments:gold.goldForms.${gold.details.form}`)

  const tourSteps: TourStep[] = [
    {
      element: '[data-testid="tour-overview"]',
      title: t('investments:gold.tour.overviewTitle'),
      description: t('investments:gold.tour.overviewBody'),
    },
    {
      element: '[data-testid="investment-headline"]',
      title: t('investments:gold.tour.headlineTitle'),
      description: t('investments:gold.tour.headlineBody'),
    },
    {
      element: '[data-testid="tour-actions"]',
      title: t('investments:gold.tour.actionsTitle'),
      description: t('investments:gold.tour.actionsBody'),
    },
    {
      element: '[data-testid="tour-details"]',
      title: t('investments:gold.tour.detailsTitle'),
      description: t('investments:gold.tour.detailsBody'),
    },
    {
      element: '[data-testid="tour-chart"]',
      title: t('investments:gold.tour.chartTitle'),
      description: t('investments:gold.tour.chartBody'),
    },
    {
      element: '[data-testid="tour-snapshots"]',
      title: t('investments:gold.tour.snapshotsTitle'),
      description: t('investments:gold.tour.snapshotsBody'),
    },
    {
      element: '[data-testid="tour-transactions"]',
      title: t('investments:gold.tour.transactionsTitle'),
      description: t('investments:gold.tour.transactionsBody'),
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
            {gold.investment.display_name}
          </h1>
          <p className="text-sm text-muted-foreground">
            {formLabel} · {formatGoldPurity(gold.details.purity)}
          </p>
          <InvestmentHeadline
            currency={gold.investment.native_currency}
            latestValue={latestSnapshot ? Number(latestSnapshot.amount) : null}
            totalCost={computeCostBasis(transactions ?? []).cost}
            status={gold.investment.status}
            terminatedAt={gold.investment.terminated_at}
          />
          <DetailTagControl group="investment" positionId={gold.investment.id} currentTagId={gold.investment.tag_id} />
        </div>
        <div data-testid="tour-actions" className="flex gap-2">
          <HelpTourButton steps={tourSteps} />
          <Button variant="outline" size="sm" onClick={() => setEditOpen(true)}>
            <Pencil className="mr-1 size-4" />
            {t('common:actions.edit')}
          </Button>
          <TerminatePositionDialog
            group="investments"
            id={gold.investment.id}
            listKey="golds"
            currentStatus={gold.investment.status}
            currentTerminatedAt={gold.investment.terminated_at}
            currentNote={gold.investment.termination_note}
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
          <CardTitle>{t('investments:gold.detailsCardTitle')}</CardTitle>
          <CardDescription>
            {t('investments:gold.detailsCardLine', {
              ownership: ownershipLabel(
                gold.investment.ownership_type,
                gold.investment.sole_owner_user_id,
                members,
                currentUser,
              ),
              currency: gold.investment.native_currency,
            })}{' '}
            <StatusBadge group="investments" status={gold.investment.status} />
          </CardDescription>
        </CardHeader>
        {gold.investment.description && (
          <CardContent className="text-sm">
            {gold.investment.description}
          </CardContent>
        )}
      </Card>

      {snapshots && snapshots.length >= 2 && (
        <Card data-testid="tour-chart">
          <CardHeader>
            <CardTitle>{t('investments:snapshotsCard.chartTitle')}</CardTitle>
            <CardDescription>
              {t('investments:snapshotsCard.chartDescription', {
                currency: gold.investment.native_currency,
              })}
            </CardDescription>
          </CardHeader>
          <CardContent>
            <SnapshotChart
              snapshots={snapshots}
              currency={gold.investment.native_currency}
              costSeries={costBasisSeries(snapshots, transactions ?? [])}
              status={gold.investment.status}
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
                {t('investments:gold.snapshotsDescription')}
              </CardDescription>
            </div>
            <div className="flex flex-wrap gap-2">
              {/* Full position workbook (Detail + Snapshots + Transactions);
                  available regardless of status. */}
              <Button asChild size="sm" variant="outline" data-testid="gold-export">
                <a href={goldExportUrl(gold.investment.id)}>
                  <Download className="mr-1 size-4" />
                  {t('common:export.trigger')}
                </a>
              </Button>
              {isActiveStatus(gold.investment.status) && (
                <>
                  <CreateQuantityPriceSnapshotDialog
                    currency={gold.investment.native_currency}
                    priceHint={t('investments:gold.snapshotPriceHint')}
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
                    templateUrl={investmentImportTemplateUrl(gold.investment.id)}
                    mutation={importSnapshotMutation}
                    currency={gold.investment.native_currency}
                  />
                </>
              )}
            </div>
          </div>
        </CardHeader>
        <CardContent className="p-0">
          {!snapshots || snapshots.length === 0 ? (
            <p className="p-6 text-sm text-muted-foreground">
              {t('investments:gold.snapshotsEmpty')}
            </p>
          ) : (
            <>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('investments:snapshotsCard.monthHeader')}</TableHead>
                    <TableHead className="text-right">{t('investments:gold.gramsHeader')}</TableHead>
                    <TableHead className="text-right">{t('investments:gold.pricePerGramHeader')}</TableHead>
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
                {t('investments:gold.transactionsDescription')}
              </CardDescription>
            </div>
            {isActiveStatus(gold.investment.status) && (
              <div className="flex flex-wrap gap-2">
                <CreateTradeTransactionDialog
                  currency={gold.investment.native_currency}
                  txnType="buy"
                  quantityUnit={quantityUnit}
                  priceHint={t('investments:gold.buyPriceHint')}
                  mutation={createTransactionMutation}
                />
                <CreateTradeTransactionDialog
                  currency={gold.investment.native_currency}
                  txnType="sell"
                  quantityUnit={quantityUnit}
                  priceHint={t('investments:gold.sellPriceHint')}
                  mutation={createTransactionMutation}
                />
                <CreateFeeTransactionDialog
                  currency={gold.investment.native_currency}
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
              {t('investments:gold.reconcileWarning', {
                actual: recon.actual,
                expected: recon.expected,
              })}
            </div>
          )}
          {!transactions || transactions.length === 0 ? (
            <p className="p-6 text-sm text-muted-foreground">
              {t('investments:gold.transactionsEmpty')}
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

      <EditGoldDialog
        key={gold.investment.id}
        open={editOpen}
        onOpenChange={setEditOpen}
        gold={gold}
      />
      <ConfirmDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        title={t('investments:gold.deleteTitle')}
        description={t('investments:gold.deleteDetailDescription')}
        confirmLabel={t('common:delete')}
        destructive
        pending={deleteMutation.isPending}
        onConfirm={handleConfirmDelete}
      />
    </div>
  )
}
