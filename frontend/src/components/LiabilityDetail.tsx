import { useState } from 'react'
import { Download, Pencil, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
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
import { useLiability, useDeleteLiability } from '@/hooks/useLiabilities'
import {
  useLiabilitySnapshots,
  useCreateLiabilitySnapshot,
  useUpdateLiabilitySnapshot,
  useDeleteLiabilitySnapshot,
  useImportLiabilitySnapshots,
  liabilityImportTemplateUrl,
  liabilityExportUrl,
} from '@/hooks/useLiabilitySnapshots'
import { CreateSnapshotDialog } from '@/components/CreateSnapshotDialog'
import { ImportSnapshotsDialog } from '@/components/ImportSnapshotsDialog'
import { TerminatePositionDialog } from '@/components/TerminatePositionDialog'
import { StatusBadge } from '@/components/StatusBadge'
import { isActiveStatus } from '@/lib/lifecycle'
import { EditLiabilityDialog } from '@/components/EditLiabilityDialog'
import { ConfirmDialog } from '@/components/ConfirmDialog'
import { SnapshotRow } from '@/components/SnapshotRow'
import { SnapshotChart } from '@/components/SnapshotChart'
import { HelpTourButton, type TourStep } from '@/components/HelpTourButton'
import { DetailTagControl } from '@/components/DetailTagControl'
import { useHouseholdMembers } from '@/hooks/useHouseholdMembers'
import { useSession } from '@/hooks/useSession'
import { formatCurrency, formatDate } from '@/lib/format'
import { ownershipLabel } from '@/lib/ownership'

type Props = {
  liabilityId: string
  onBack: () => void
}

const PAGE_SIZE = 12

export function LiabilityDetail({ liabilityId, onBack }: Props) {
  const { t } = useTranslation(['liabilities', 'common', 'errors'])
  const { data: liability, isPending, error } = useLiability(liabilityId)
  const { data: snapshots } = useLiabilitySnapshots(liabilityId)
  const deleteMutation = useDeleteLiability()
  const createSnapshotMutation = useCreateLiabilitySnapshot(liabilityId)
  const updateSnapshotMutation = useUpdateLiabilitySnapshot(liabilityId)
  const deleteSnapshotMutation = useDeleteLiabilitySnapshot(liabilityId)
  const importSnapshotMutation = useImportLiabilitySnapshots(liabilityId)
  const { data: members } = useHouseholdMembers()
  const { data: currentUser } = useSession()

  const [editOpen, setEditOpen] = useState(false)
  const [deleteOpen, setDeleteOpen] = useState(false)
  const [page, setPage] = useState(1)

  const totalPages = Math.max(
    1,
    Math.ceil((snapshots?.length ?? 0) / PAGE_SIZE),
  )
  const effectivePage = Math.min(page, totalPages)

  function handleConfirmDelete() {
    deleteMutation.mutate(liabilityId, {
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
  if (!liability) return null

  const pageSnapshots = (snapshots ?? []).slice(
    (effectivePage - 1) * PAGE_SIZE,
    effectivePage * PAGE_SIZE,
  )

  const hasDetails =
    liability.principal ||
    liability.interest_rate ||
    liability.start_date ||
    liability.maturity_date ||
    liability.term_months !== null ||
    liability.description

  const subtypeLabel = t(`liabilities:subtypes.${liability.subtype}`)
  const periodMissing = t('liabilities:periodMissing')

  const tourSteps: TourStep[] = [
    {
      element: '[data-testid="tour-overview"]',
      title: t('liabilities:tour.overviewTitle'),
      description: t('liabilities:tour.overviewBody'),
    },
    {
      element: '[data-testid="tour-actions"]',
      title: t('liabilities:tour.actionsTitle'),
      description: t('liabilities:tour.actionsBody'),
    },
    {
      element: '[data-testid="tour-details"]',
      title: t('liabilities:tour.detailsTitle'),
      description: t('liabilities:tour.detailsBody'),
    },
    {
      element: '[data-testid="tour-chart"]',
      title: t('liabilities:tour.chartTitle'),
      description: t('liabilities:tour.chartBody'),
    },
    {
      element: '[data-testid="tour-snapshots"]',
      title: t('liabilities:tour.snapshotsTitle'),
      description: t('liabilities:tour.snapshotsBody'),
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
            {liability.display_name}
          </h1>
          <p className="text-sm text-muted-foreground">
            {t('liabilities:detailSubtitle', {
              subtype: subtypeLabel,
              counterparty: liability.counterparty_name,
            })}
          </p>
          <DetailTagControl group="liability" positionId={liability.id} currentTagId={liability.tag_id} />
        </div>
        <div data-testid="tour-actions" className="flex gap-2">
          <HelpTourButton steps={tourSteps} />
          <Button variant="outline" size="sm" onClick={() => setEditOpen(true)}>
            <Pencil className="mr-1 size-4" />
            {t('common:actions.edit')}
          </Button>
          <TerminatePositionDialog
            group="liabilities"
            id={liability.id}
            listKey="liabilities"
            currentStatus={liability.status}
            currentTerminatedAt={liability.terminated_at}
            currentNote={liability.termination_note}
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
          <CardTitle>{t('liabilities:detailsCardTitle')}</CardTitle>
          <CardDescription>
            {t('liabilities:detailsCardLine', {
              ownership: ownershipLabel(
                liability.ownership_type,
                liability.sole_owner_user_id,
                members,
                currentUser,
              ),
              currency: liability.native_currency,
            })}{' '}
            <StatusBadge group="liabilities" status={liability.status} />
          </CardDescription>
        </CardHeader>
        {hasDetails && (
          <CardContent className="text-sm space-y-1">
            {liability.principal && (
              <p>
                <span className="text-muted-foreground">
                  {t('liabilities:principalLabel')}
                </span>{' '}
                {formatCurrency(liability.principal, liability.native_currency)}
              </p>
            )}
            {liability.interest_rate && (
              <p>
                <span className="text-muted-foreground">
                  {t('liabilities:interestRateLabel')}
                </span>{' '}
                {t('liabilities:interestRateValue', {
                  rate: Number(liability.interest_rate).toFixed(2),
                })}
              </p>
            )}
            {liability.term_months !== null && (
              <p>
                <span className="text-muted-foreground">
                  {t('liabilities:termLabel')}
                </span>{' '}
                {t('liabilities:termValue', { count: liability.term_months })}
              </p>
            )}
            {(liability.start_date || liability.maturity_date) && (
              <p>
                <span className="text-muted-foreground">
                  {t('liabilities:periodLabel')}
                </span>{' '}
                {t('liabilities:periodValue', {
                  start: liability.start_date
                    ? formatDate(liability.start_date)
                    : periodMissing,
                  end: liability.maturity_date
                    ? formatDate(liability.maturity_date)
                    : periodMissing,
                })}
              </p>
            )}
            {liability.description && <p className="pt-1">{liability.description}</p>}
          </CardContent>
        )}
      </Card>

      {snapshots && snapshots.length >= 2 && (
        <Card data-testid="tour-chart">
          <CardHeader>
            <CardTitle>{t('liabilities:chartTitle')}</CardTitle>
            <CardDescription>
              {t('liabilities:chartDescription', {
                currency: liability.native_currency,
              })}
            </CardDescription>
          </CardHeader>
          <CardContent>
            <SnapshotChart
              snapshots={snapshots}
              currency={liability.native_currency}
            />
          </CardContent>
        </Card>
      )}

      <Card data-testid="tour-snapshots">
        <CardHeader>
          <div className="flex items-center justify-between gap-4">
            <div>
              <CardTitle>{t('liabilities:snapshotsTitle')}</CardTitle>
              <CardDescription>
                {t('liabilities:snapshotsDescription')}
              </CardDescription>
            </div>
            <div className="flex flex-wrap gap-2">
              {/* Export the full position workbook (Detail + Snapshots). Plain
                  anchor download — session cookie rides along same-origin, like
                  the import template link. Available regardless of status so a
                  terminated position can still be backed up. */}
              <Button
                asChild
                size="sm"
                variant="outline"
                data-testid="liability-export"
              >
                <a href={liabilityExportUrl(liability.id)}>
                  <Download className="mr-1 size-4" />
                  {t('common:export.trigger')}
                </a>
              </Button>
              {isActiveStatus(liability.status) && (
                <>
                  <CreateSnapshotDialog
                    currency={liability.native_currency}
                    mutation={createSnapshotMutation}
                    carryover={
                      snapshots?.[0]
                        ? {
                            amount: snapshots[0].amount,
                            lastSnapshotMonth: snapshots[0].year_month,
                          }
                        : null
                    }
                  />
                  <ImportSnapshotsDialog
                    templateUrl={liabilityImportTemplateUrl(liability.id)}
                    mutation={importSnapshotMutation}
                    currency={liability.native_currency}
                  />
                </>
              )}
            </div>
          </div>
        </CardHeader>
        <CardContent className="p-0">
          {!snapshots || snapshots.length === 0 ? (
            <p className="p-6 text-sm text-muted-foreground">
              {t('liabilities:snapshotsEmpty')}
            </p>
          ) : (
            <>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('common:tableHeaders.month')}</TableHead>
                    <TableHead>{t('common:tableHeaders.amount')}</TableHead>
                    <TableHead>{t('common:tableHeaders.notes')}</TableHead>
                    <TableHead className="w-12"></TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {pageSnapshots.map((s) => (
                    <SnapshotRow
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

      <EditLiabilityDialog
        key={liability.id}
        open={editOpen}
        onOpenChange={setEditOpen}
        liability={liability}
      />
      <ConfirmDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        title={t('liabilities:deleteTitle')}
        description={t('liabilities:deleteDetailDescription')}
        confirmLabel={t('common:delete')}
        destructive
        pending={deleteMutation.isPending}
        onConfirm={handleConfirmDelete}
      />
    </div>
  )
}
