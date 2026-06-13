import { useState } from 'react'
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
import {
  useTimeDeposit,
  useDeleteTimeDeposit,
} from '@/hooks/useInvestments'
import {
  useInvestmentSnapshots,
  useCreateInvestmentSnapshot,
  useUpdateInvestmentSnapshot,
  useDeleteInvestmentSnapshot,
  useImportInvestmentSnapshots,
  investmentImportTemplateUrl,
  timeDepositExportUrl,
} from '@/hooks/useInvestmentSnapshots'
import {
  useInvestmentTransactions,
  useCreateInvestmentTransaction,
  useUpdateInvestmentTransaction,
  useDeleteInvestmentTransaction,
} from '@/hooks/useInvestmentTransactions'
import { CreateAccruedInterestSnapshotDialog } from '@/components/CreateAccruedInterestSnapshotDialog'
import { ImportSnapshotsDialog } from '@/components/ImportSnapshotsDialog'
import { CreateMaturityTransactionDialog } from '@/components/CreateMaturityTransactionDialog'
import { CreateTimeDepositDialog } from '@/components/CreateTimeDepositDialog'
import { LinkRolloverSuccessorDialog } from '@/components/LinkRolloverSuccessorDialog'
import { TransactionRow } from '@/components/TransactionRow'
import { TerminatePositionDialog } from '@/components/TerminatePositionDialog'
import { StatusBadge } from '@/components/StatusBadge'
import { isActiveStatus } from '@/lib/lifecycle'
import { EditTimeDepositDialog } from '@/components/EditTimeDepositDialog'
import { ConfirmDialog } from '@/components/ConfirmDialog'
import { AccruedInterestSnapshotRow } from '@/components/AccruedInterestSnapshotRow'
import { SnapshotChart } from '@/components/SnapshotChart'
import { HelpTourButton, type TourStep } from '@/components/HelpTourButton'
import { DetailTagControl } from '@/components/DetailTagControl'
import { useHouseholdMembers } from '@/hooks/useHouseholdMembers'
import { useSession } from '@/hooks/useSession'
import { formatCurrency, formatDate } from '@/lib/format'
import { ownershipLabel } from '@/lib/ownership'
import { matchesTxnSearch } from '@/lib/transactionSearch'
import { maturityRolloverPrefill } from '@/lib/rollover'
import { flatCostSeries } from '@/lib/costBasis'
import { ArrowDown, ArrowUp, Download, Pencil, Repeat, Trash2 } from 'lucide-react'
import { InvestmentHeadline } from '@/components/InvestmentHeadline'

type Props = {
  investmentId: string
  onBack: () => void
  // Navigate to another time deposit's detail (the rollover-chain links). Kept
  // as a callback so the screen stays router-unaware (App.tsx bridges to nav).
  onSelectTimeDeposit: (id: string) => void
}

const PAGE_SIZE = 12

export function TimeDepositDetail({
  investmentId,
  onBack,
  onSelectTimeDeposit,
}: Props) {
  const { t } = useTranslation(['investments', 'common', 'errors'])
  const { data: td, isPending, error } = useTimeDeposit(investmentId)
  const { data: snapshots } = useInvestmentSnapshots(investmentId)
  const { data: transactions } = useInvestmentTransactions(investmentId)
  const deleteMutation = useDeleteTimeDeposit()
  const createSnapshotMutation = useCreateInvestmentSnapshot(
    investmentId,
    'time-deposits',
  )
  const updateSnapshotMutation = useUpdateInvestmentSnapshot(
    investmentId,
    'time-deposits',
  )
  const deleteSnapshotMutation = useDeleteInvestmentSnapshot(
    investmentId,
    'time-deposits',
  )
  const importSnapshotMutation = useImportInvestmentSnapshots(
    investmentId,
    'time-deposits',
  )
  const createTransactionMutation = useCreateInvestmentTransaction(
    investmentId,
    'time-deposits',
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
  if (!td) return null

  const pageSnapshots = (snapshots ?? []).slice(
    (effectivePage - 1) * PAGE_SIZE,
    effectivePage * PAGE_SIZE,
  )
  const pageTransactions = filteredTransactions.slice(
    (effectiveTxnPage - 1) * PAGE_SIZE,
    effectiveTxnPage * PAGE_SIZE,
  )
  // Maturity is uniquely terminal: posting it flips the position to 'matured'
  // (backend hard guard, ADR-0009), after which the transaction-create row is
  // gated off entirely by isActiveStatus below.
  const ratePct = Number(td.details.interest_rate).toFixed(2)
  const rolloverLabel = t(
    `investments:timeDeposit.rolloverPolicy.${td.details.rollover_policy}`,
  )
  // Q14c-iv: a matured TD whose principal/interest rolled over needs a fresh
  // deposit to hold the rolled funds. Non-null only when something actually
  // rolled (and a maturity txn exists, which only happens once matured).
  const rollover = maturityRolloverPrefill(td, transactions)

  const tourSteps: TourStep[] = [
    {
      element: '[data-testid="tour-overview"]',
      title: t('investments:timeDeposit.tour.overviewTitle'),
      description: t('investments:timeDeposit.tour.overviewBody'),
    },
    {
      element: '[data-testid="investment-headline"]',
      title: t('investments:timeDeposit.tour.headlineTitle'),
      description: t('investments:timeDeposit.tour.headlineBody'),
    },
    {
      element: '[data-testid="tour-actions"]',
      title: t('investments:timeDeposit.tour.actionsTitle'),
      description: t('investments:timeDeposit.tour.actionsBody'),
    },
    {
      element: '[data-testid="tour-details"]',
      title: t('investments:timeDeposit.tour.detailsTitle'),
      description: t('investments:timeDeposit.tour.detailsBody'),
    },
    {
      element: '[data-testid="tour-chart"]',
      title: t('investments:timeDeposit.tour.chartTitle'),
      description: t('investments:timeDeposit.tour.chartBody'),
    },
    {
      element: '[data-testid="tour-snapshots"]',
      title: t('investments:timeDeposit.tour.snapshotsTitle'),
      description: t('investments:timeDeposit.tour.snapshotsBody'),
    },
    {
      element: '[data-testid="tour-transactions"]',
      title: t('investments:timeDeposit.tour.transactionsTitle'),
      description: t('investments:timeDeposit.tour.transactionsBody'),
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
            {td.investment.display_name}
          </h1>
          <p className="text-sm text-muted-foreground">
            {t('investments:timeDeposit.subtitle', {
              bank: td.details.bank_name,
              rate: ratePct,
              months: td.details.term_months,
            })}
          </p>
          <InvestmentHeadline
            currency={td.investment.native_currency}
            latestValue={
              snapshots && snapshots.length > 0
                ? Number(snapshots[0].amount)
                : null
            }
            totalCost={Number(td.details.principal)}
            status={td.investment.status}
            terminatedAt={td.investment.terminated_at}
          />
          <DetailTagControl group="investment" positionId={td.investment.id} currentTagId={td.investment.tag_id} />
        </div>
        <div data-testid="tour-actions" className="flex gap-2">
          <HelpTourButton steps={tourSteps} />
          <Button variant="outline" size="sm" onClick={() => setEditOpen(true)}>
            <Pencil className="mr-1 size-4" />
            {t('common:actions.edit')}
          </Button>
          <TerminatePositionDialog
            group="investments"
            id={td.investment.id}
            listKey="time-deposits"
            currentStatus={td.investment.status}
            currentTerminatedAt={td.investment.terminated_at}
            currentNote={td.investment.termination_note}
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

      {rollover && (
        <div
          data-testid="rollover-callout"
          className="flex flex-wrap items-center justify-between gap-3 rounded-md border border-sky-200 bg-sky-50 px-4 py-3 text-sky-900"
        >
          <div className="flex items-start gap-3">
            <Repeat className="mt-0.5 size-5 shrink-0 text-sky-600" />
            <div className="text-sm">
              <p className="font-medium">
                {t('investments:timeDeposit.rollover.calloutTitle')}
              </p>
              <p className="text-sky-800">
                {t('investments:timeDeposit.rollover.calloutBody', {
                  amount: formatCurrency(
                    rollover.rolledAmount.toString(),
                    td.investment.native_currency,
                  ),
                })}
              </p>
            </div>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <CreateTimeDepositDialog
              prefill={rollover.prefill}
              rolledFromInvestmentId={td.investment.id}
              triggerLabel={t('investments:timeDeposit.rollover.calloutAction')}
            />
            {/* Hand-created the successor already? Link it so this callout
                clears without spawning a duplicate (issue #65). */}
            <LinkRolloverSuccessorDialog sourceId={td.investment.id} />
          </div>
        </div>
      )}

      <Card data-testid="tour-details">
        <CardHeader>
          <CardTitle>{t('investments:timeDeposit.detailsCardTitle')}</CardTitle>
          <CardDescription>
            {t('investments:timeDeposit.detailsCardLine', {
              ownership: ownershipLabel(
                td.investment.ownership_type,
                td.investment.sole_owner_user_id,
                members,
                currentUser,
              ),
              currency: td.investment.native_currency,
            })}{' '}
            <StatusBadge group="investments" status={td.investment.status} />
          </CardDescription>
        </CardHeader>
        <CardContent className="text-sm space-y-1">
          <p>
            <span className="text-muted-foreground">
              {t('investments:timeDeposit.principalLabel')}
            </span>{' '}
            {formatCurrency(
              td.details.principal,
              td.investment.native_currency,
            )}
          </p>
          <p>
            <span className="text-muted-foreground">
              {t('investments:timeDeposit.placementLabel')}
            </span>{' '}
            {formatDate(td.details.placement_date)}
          </p>
          <p>
            <span className="text-muted-foreground">
              {t('investments:timeDeposit.maturityLabel')}
            </span>{' '}
            {formatDate(td.details.maturity_date)}
          </p>
          <p>
            <span className="text-muted-foreground">
              {t('investments:timeDeposit.atMaturityLabel')}
            </span>{' '}
            {rolloverLabel}
          </p>
          {td.investment.description && (
            <p className="pt-1">{td.investment.description}</p>
          )}
        </CardContent>
      </Card>

      {(td.rolled_from || td.rolled_to) && (
        <Card data-testid="rollover-card">
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Repeat className="size-4 text-muted-foreground" />
              {t('investments:timeDeposit.rolloverChain.title')}
            </CardTitle>
            <CardDescription>
              {t('investments:timeDeposit.rolloverChain.description')}
            </CardDescription>
          </CardHeader>
          <CardContent className="text-sm space-y-2">
            <div className="flex items-center gap-2">
              <ArrowUp className="size-4 shrink-0 text-muted-foreground" />
              <span className="text-muted-foreground">
                {t('investments:timeDeposit.rolloverChain.fromLabel')}
              </span>
              {td.rolled_from ? (
                <button
                  type="button"
                  data-testid="rollover-from-link"
                  className="font-medium text-sky-700 underline underline-offset-2 hover:text-sky-900"
                  onClick={() => onSelectTimeDeposit(td.rolled_from!.id)}
                >
                  {td.rolled_from.display_name}
                </button>
              ) : (
                <span className="text-muted-foreground">
                  {t('investments:timeDeposit.rolloverChain.none')}
                </span>
              )}
            </div>
            <div className="flex items-center gap-2">
              <ArrowDown className="size-4 shrink-0 text-muted-foreground" />
              <span className="text-muted-foreground">
                {t('investments:timeDeposit.rolloverChain.intoLabel')}
              </span>
              {td.rolled_to ? (
                <button
                  type="button"
                  data-testid="rollover-into-link"
                  className="font-medium text-sky-700 underline underline-offset-2 hover:text-sky-900"
                  onClick={() => onSelectTimeDeposit(td.rolled_to!.id)}
                >
                  {td.rolled_to.display_name}
                </button>
              ) : (
                <span className="text-muted-foreground">
                  {t('investments:timeDeposit.rolloverChain.noneYet')}
                </span>
              )}
            </div>
          </CardContent>
        </Card>
      )}

      {snapshots && snapshots.length >= 2 && (
        <Card data-testid="tour-chart">
          <CardHeader>
            <CardTitle>{t('investments:snapshotsCard.chartTitle')}</CardTitle>
            <CardDescription>
              {t('investments:snapshotsCard.chartDescriptionTotal', {
                currency: td.investment.native_currency,
              })}
            </CardDescription>
          </CardHeader>
          <CardContent>
            <SnapshotChart
              snapshots={snapshots}
              currency={td.investment.native_currency}
              costSeries={flatCostSeries(
                snapshots,
                Number(td.details.principal),
              )}
              status={td.investment.status}
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
                {t('investments:timeDeposit.snapshotsDescription')}
              </CardDescription>
            </div>
            <div className="flex flex-wrap gap-2">
              {/* Full position workbook (Detail + Snapshots + Transactions);
                  available regardless of status. */}
              <Button asChild size="sm" variant="outline" data-testid="time-deposit-export">
                <a href={timeDepositExportUrl(td.investment.id)}>
                  <Download className="mr-1 size-4" />
                  {t('common:export.trigger')}
                </a>
              </Button>
              {isActiveStatus(td.investment.status) && (
                <>
                  <CreateAccruedInterestSnapshotDialog
                    currency={td.investment.native_currency}
                    mutation={createSnapshotMutation}
                    carryover={
                      snapshots?.[0]
                        ? {
                            amount: snapshots[0].amount,
                            accrued_interest: snapshots[0].accrued_interest,
                          }
                        : null
                    }
                  />
                  <ImportSnapshotsDialog
                    templateUrl={investmentImportTemplateUrl(td.investment.id)}
                    mutation={importSnapshotMutation}
                    currency={td.investment.native_currency}
                  />
                </>
              )}
            </div>
          </div>
        </CardHeader>
        <CardContent className="p-0">
          {!snapshots || snapshots.length === 0 ? (
            <p className="p-6 text-sm text-muted-foreground">
              {t('investments:timeDeposit.snapshotsEmpty')}
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
                {t('investments:timeDeposit.transactionsDescription')}
              </CardDescription>
            </div>
            {isActiveStatus(td.investment.status) && (
              <div className="flex flex-wrap gap-2">
                <CreateMaturityTransactionDialog
                  currency={td.investment.native_currency}
                  rolloverPolicy={td.details.rollover_policy}
                  mutation={createTransactionMutation}
                />
              </div>
            )}
          </div>
        </CardHeader>
        <CardContent className="p-0">
          {!transactions || transactions.length === 0 ? (
            <p className="p-6 text-sm text-muted-foreground">
              {t('investments:timeDeposit.transactionsEmpty')}
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
                          quantityUnit=""
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

      <EditTimeDepositDialog
        key={td.investment.id}
        open={editOpen}
        onOpenChange={setEditOpen}
        timeDeposit={td}
      />
      <ConfirmDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        title={t('investments:timeDeposit.deleteTitle')}
        description={t('investments:timeDeposit.deleteDetailDescription')}
        confirmLabel={t('common:delete')}
        destructive
        pending={deleteMutation.isPending}
        onConfirm={handleConfirmDelete}
      />
    </div>
  )
}
