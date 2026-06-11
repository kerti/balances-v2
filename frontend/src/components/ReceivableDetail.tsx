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
import { useReceivable, useDeleteReceivable } from '@/hooks/useReceivables'
import {
  useReceivableSnapshots,
  useCreateReceivableSnapshot,
  useUpdateReceivableSnapshot,
  useDeleteReceivableSnapshot,
  useImportReceivableSnapshots,
  receivableImportTemplateUrl,
  receivableExportUrl,
} from '@/hooks/useReceivableSnapshots'
import { CreateSnapshotDialog } from '@/components/CreateSnapshotDialog'
import { ImportSnapshotsDialog } from '@/components/ImportSnapshotsDialog'
import { TerminatePositionDialog } from '@/components/TerminatePositionDialog'
import { StatusBadge } from '@/components/StatusBadge'
import { isActiveStatus } from '@/lib/lifecycle'
import { EditReceivableDialog } from '@/components/EditReceivableDialog'
import { ConfirmDialog } from '@/components/ConfirmDialog'
import { SnapshotRow } from '@/components/SnapshotRow'
import { SnapshotChart } from '@/components/SnapshotChart'
import { HelpTourButton, type TourStep } from '@/components/HelpTourButton'
import { DetailTagControl } from '@/components/DetailTagControl'
import { useHouseholdMembers } from '@/hooks/useHouseholdMembers'
import { useSession } from '@/hooks/useSession'
import { formatDate } from '@/lib/format'
import { ownershipLabel } from '@/lib/ownership'

type Props = {
  receivableId: string
  onBack: () => void
}

const PAGE_SIZE = 12

export function ReceivableDetail({ receivableId, onBack }: Props) {
  const { t } = useTranslation(['receivables', 'common', 'errors'])
  const { data: receivable, isPending, error } = useReceivable(receivableId)
  const { data: snapshots } = useReceivableSnapshots(receivableId)
  const deleteMutation = useDeleteReceivable()
  const createSnapshotMutation = useCreateReceivableSnapshot(receivableId)
  const updateSnapshotMutation = useUpdateReceivableSnapshot(receivableId)
  const deleteSnapshotMutation = useDeleteReceivableSnapshot(receivableId)
  const importSnapshotMutation = useImportReceivableSnapshots(receivableId)
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
    deleteMutation.mutate(receivableId, {
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
  if (!receivable) return null

  const pageSnapshots = (snapshots ?? []).slice(
    (effectivePage - 1) * PAGE_SIZE,
    effectivePage * PAGE_SIZE,
  )

  const tourSteps: TourStep[] = [
    {
      element: '[data-testid="tour-overview"]',
      title: t('receivables:tour.overviewTitle'),
      description: t('receivables:tour.overviewBody'),
    },
    {
      element: '[data-testid="tour-actions"]',
      title: t('receivables:tour.actionsTitle'),
      description: t('receivables:tour.actionsBody'),
    },
    {
      element: '[data-testid="tour-details"]',
      title: t('receivables:tour.detailsTitle'),
      description: t('receivables:tour.detailsBody'),
    },
    {
      element: '[data-testid="tour-chart"]',
      title: t('receivables:tour.chartTitle'),
      description: t('receivables:tour.chartBody'),
    },
    {
      element: '[data-testid="tour-snapshots"]',
      title: t('receivables:tour.snapshotsTitle'),
      description: t('receivables:tour.snapshotsBody'),
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
            {receivable.display_name}
          </h1>
          <p className="text-sm text-muted-foreground">
            {receivable.due_date
              ? t('receivables:detailSubtitleWithDue', {
                  counterparty: receivable.counterparty_name,
                  date: formatDate(receivable.due_date),
                })
              : receivable.counterparty_name}
          </p>
          <DetailTagControl group="receivable" positionId={receivable.id} currentTagId={receivable.tag_id} />
        </div>
        <div data-testid="tour-actions" className="flex gap-2">
          <HelpTourButton steps={tourSteps} />
          <Button variant="outline" size="sm" onClick={() => setEditOpen(true)}>
            <Pencil className="mr-1 size-4" />
            {t('common:actions.edit')}
          </Button>
          <TerminatePositionDialog
            group="receivables"
            id={receivable.id}
            listKey="receivables"
            currentStatus={receivable.status}
            currentTerminatedAt={receivable.terminated_at}
            currentNote={receivable.termination_note}
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
          <CardTitle>{t('receivables:detailsCardTitle')}</CardTitle>
          <CardDescription>
            {t('receivables:detailsCardLine', {
              ownership: ownershipLabel(
                receivable.ownership_type,
                receivable.sole_owner_user_id,
                members,
                currentUser,
              ),
              currency: receivable.native_currency,
            })}{' '}
            <StatusBadge group="receivables" status={receivable.status} />
          </CardDescription>
        </CardHeader>
        {receivable.description && (
          <CardContent>
            <p className="text-sm">{receivable.description}</p>
          </CardContent>
        )}
      </Card>

      {snapshots && snapshots.length >= 2 && (
        <Card data-testid="tour-chart">
          <CardHeader>
            <CardTitle>{t('receivables:chartTitle')}</CardTitle>
            <CardDescription>
              {t('receivables:chartDescription', {
                currency: receivable.native_currency,
              })}
            </CardDescription>
          </CardHeader>
          <CardContent>
            <SnapshotChart
              snapshots={snapshots}
              currency={receivable.native_currency}
            />
          </CardContent>
        </Card>
      )}

      <Card data-testid="tour-snapshots">
        <CardHeader>
          <div className="flex items-center justify-between gap-4">
            <div>
              <CardTitle>{t('receivables:snapshotsTitle')}</CardTitle>
              <CardDescription>
                {t('receivables:snapshotsDescription')}
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
                data-testid="receivable-export"
              >
                <a href={receivableExportUrl(receivable.id)}>
                  <Download className="mr-1 size-4" />
                  {t('common:export.trigger')}
                </a>
              </Button>
              {isActiveStatus(receivable.status) && (
                <>
                  <CreateSnapshotDialog
                    currency={receivable.native_currency}
                    mutation={createSnapshotMutation}
                  />
                  <ImportSnapshotsDialog
                    templateUrl={receivableImportTemplateUrl(receivable.id)}
                    mutation={importSnapshotMutation}
                    currency={receivable.native_currency}
                  />
                </>
              )}
            </div>
          </div>
        </CardHeader>
        <CardContent className="p-0">
          {!snapshots || snapshots.length === 0 ? (
            <p className="p-6 text-sm text-muted-foreground">
              {t('receivables:snapshotsEmpty')}
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

      <EditReceivableDialog
        key={receivable.id}
        open={editOpen}
        onOpenChange={setEditOpen}
        receivable={receivable}
      />
      <ConfirmDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        title={t('receivables:deleteTitle')}
        description={t('receivables:deleteDetailDescription')}
        confirmLabel={t('common:delete')}
        destructive
        pending={deleteMutation.isPending}
        onConfirm={handleConfirmDelete}
      />
    </div>
  )
}
