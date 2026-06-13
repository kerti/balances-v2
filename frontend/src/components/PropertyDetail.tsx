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
import { useProperty, useDeleteProperty } from '@/hooks/useProperties'
import {
  useSnapshots,
  useCreateSnapshot,
  useUpdateSnapshot,
  useDeleteSnapshot,
  useImportSnapshots,
  importTemplateUrl,
  propertyExportUrl,
} from '@/hooks/useAssetSnapshots'
import { CreateSnapshotDialog } from '@/components/CreateSnapshotDialog'
import { ImportSnapshotsDialog } from '@/components/ImportSnapshotsDialog'
import { TerminatePositionDialog } from '@/components/TerminatePositionDialog'
import { StatusBadge } from '@/components/StatusBadge'
import { isActiveStatus } from '@/lib/lifecycle'
import { EditPropertyDialog } from '@/components/EditPropertyDialog'
import { ConfirmDialog } from '@/components/ConfirmDialog'
import { SnapshotRow } from '@/components/SnapshotRow'
import { SnapshotChart } from '@/components/SnapshotChart'
import { HelpTourButton, type TourStep } from '@/components/HelpTourButton'
import { DetailTagControl } from '@/components/DetailTagControl'
import { useHouseholdMembers } from '@/hooks/useHouseholdMembers'
import { useSession } from '@/hooks/useSession'
import { formatCurrency, formatDate, formatSignedPercent } from '@/lib/format'
import { ownershipLabel } from '@/lib/ownership'
import { suggestRevalued } from '@/lib/revaluation'

type Props = {
  assetId: string
  onBack: () => void
}

const PAGE_SIZE = 12

export function PropertyDetail({ assetId, onBack }: Props) {
  const { t } = useTranslation(['assets', 'common', 'errors'])
  const { data: property, isPending, error } = useProperty(assetId)
  const { data: snapshots } = useSnapshots(assetId)
  const deleteMutation = useDeleteProperty()
  const createSnapshotMutation = useCreateSnapshot(assetId)
  const updateSnapshotMutation = useUpdateSnapshot(assetId)
  const deleteSnapshotMutation = useDeleteSnapshot(assetId)
  const importSnapshotMutation = useImportSnapshots(assetId)
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
    deleteMutation.mutate(assetId, {
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
  if (!property) return null

  const { asset, details } = property
  const ownerLabel = ownershipLabel(
    asset.ownership_type,
    asset.sole_owner_user_id,
    members,
    currentUser,
  )
  const pageSnapshots = (snapshots ?? []).slice(
    (effectivePage - 1) * PAGE_SIZE,
    effectivePage * PAGE_SIZE,
  )
  const typeLabel = t(`assets:property.propertyTypes.${details.property_type}`)

  const tourSteps: TourStep[] = [
    {
      element: '[data-testid="tour-overview"]',
      title: t('assets:property.tour.overviewTitle'),
      description: t('assets:property.tour.overviewBody'),
    },
    {
      element: '[data-testid="tour-actions"]',
      title: t('assets:property.tour.actionsTitle'),
      description: t('assets:property.tour.actionsBody'),
    },
    {
      element: '[data-testid="tour-details"]',
      title: t('assets:property.tour.detailsTitle'),
      description: t('assets:property.tour.detailsBody'),
    },
    {
      element: '[data-testid="tour-chart"]',
      title: t('assets:property.tour.chartTitle'),
      description: t('assets:property.tour.chartBody'),
    },
    {
      element: '[data-testid="tour-snapshots"]',
      title: t('assets:property.tour.snapshotsTitle'),
      description: t('assets:property.tour.snapshotsBody'),
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
            {asset.display_name}
          </h1>
          <p className="text-sm text-muted-foreground">
            {typeLabel}
            {details.address && ` · ${details.address}`}
          </p>
          <DetailTagControl group="asset" positionId={property.asset.id} currentTagId={property.asset.tag_id} />
        </div>
        <div data-testid="tour-actions" className="flex gap-2">
          <HelpTourButton steps={tourSteps} />
          <Button variant="outline" size="sm" onClick={() => setEditOpen(true)}>
            <Pencil className="mr-1 size-4" />
            {t('common:actions.edit')}
          </Button>
          <TerminatePositionDialog
            group="assets"
            id={asset.id}
            listKey="properties"
            currentStatus={asset.status}
            currentTerminatedAt={asset.terminated_at}
            currentNote={asset.termination_note}
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
          <CardTitle>{t('assets:property.detailsCardTitle')}</CardTitle>
          <CardDescription>
            {t('assets:property.detailsCardLine', {
              ownership: ownerLabel,
              currency: asset.native_currency,
            })}{' '}
            <StatusBadge group="assets" status={asset.status} />
          </CardDescription>
        </CardHeader>
        <CardContent className="text-sm space-y-1">
          {details.acquisition_date && (
            <p>
              {details.acquisition_cost ? (
                t('assets:property.acquiredForLine', {
                  date: formatDate(details.acquisition_date),
                  cost: formatCurrency(
                    details.acquisition_cost,
                    asset.native_currency,
                  ),
                })
              ) : (
                <>
                  <span className="text-muted-foreground">
                    {t('assets:property.acquiredLine')}
                  </span>{' '}
                  {formatDate(details.acquisition_date)}
                </>
              )}
            </p>
          )}
          {details.annual_appreciation_rate && (
            <p>
              <span className="text-muted-foreground">
                {t('assets:property.appreciationRateLabel')}
              </span>{' '}
              {t('assets:property.appreciationRateValue', {
                value: formatSignedPercent(details.annual_appreciation_rate),
              })}
            </p>
          )}
          {asset.description && (
            <p className="pt-1">{asset.description}</p>
          )}
        </CardContent>
      </Card>

      {snapshots && snapshots.length >= 2 && (
        <Card data-testid="tour-chart">
          <CardHeader>
            <CardTitle>{t('assets:property.chartTitle')}</CardTitle>
            <CardDescription>
              {t('assets:property.chartDescription', {
                currency: asset.native_currency,
              })}
            </CardDescription>
          </CardHeader>
          <CardContent>
            <SnapshotChart
              snapshots={snapshots}
              currency={asset.native_currency}
            />
          </CardContent>
        </Card>
      )}

      <Card data-testid="tour-snapshots">
        <CardHeader>
          <div className="flex items-center justify-between gap-4">
            <div>
              <CardTitle>{t('assets:property.snapshotsTitle')}</CardTitle>
              <CardDescription>
                {t('assets:property.snapshotsDescription')}
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
                data-testid="property-export"
              >
                <a href={propertyExportUrl(asset.id)}>
                  <Download className="mr-1 size-4" />
                  {t('common:export.trigger')}
                </a>
              </Button>
              {isActiveStatus(asset.status) && (
                <>
                  <CreateSnapshotDialog
                    currency={asset.native_currency}
                    mutation={createSnapshotMutation}
                    suggest={(yearMonth) =>
                      suggestRevalued({
                        newYearMonth: yearMonth,
                        annualRatePct: details.annual_appreciation_rate,
                        snapshots,
                      })
                    }
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
                    templateUrl={importTemplateUrl(asset.id)}
                    mutation={importSnapshotMutation}
                    currency={asset.native_currency}
                  />
                </>
              )}
            </div>
          </div>
        </CardHeader>
        <CardContent className="p-0">
          {!snapshots || snapshots.length === 0 ? (
            <p className="p-6 text-sm text-muted-foreground">
              {t('assets:property.snapshotsEmpty')}
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

      <EditPropertyDialog
        key={property.asset.id}
        open={editOpen}
        onOpenChange={setEditOpen}
        property={property}
      />
      <ConfirmDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        title={t('assets:property.deleteTitle')}
        description={t('assets:property.deleteDetailDescription')}
        confirmLabel={t('common:delete')}
        destructive
        pending={deleteMutation.isPending}
        onConfirm={handleConfirmDelete}
      />
    </div>
  )
}
