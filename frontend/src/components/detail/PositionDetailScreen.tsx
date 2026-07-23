import { useState } from "react";
import { Download, Pencil, Trash2 } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { TerminatePositionDialog } from "@/components/dialogs/TerminatePositionDialog";
import { ConfirmDialog } from "@/components/dialogs/ConfirmDialog";
import { StatusBadge } from "@/components/common/StatusBadge";
import { DetailTagControl } from "@/components/common/DetailTagControl";
import { SnapshotChart } from "@/components/charts/SnapshotChart";
import { HelpTourButton, type TourStep } from "@/components/shell/HelpTourButton";
import { InfoGrid } from "@/components/detail/InfoGrid";
import { HistorySection } from "@/components/detail/HistorySection";
import {
  useSnapshots,
  useCreateSnapshot,
  useUpdateSnapshot,
  useDeleteSnapshot,
  useImportSnapshots,
} from "@/hooks/useAssetSnapshots";
import { useHouseholdMembers } from "@/hooks/useHouseholdMembers";
import { useSession } from "@/hooks/useSession";
import { isActiveStatus } from "@/lib/lifecycle";
import { ownershipLabel } from "@/lib/ownership";
import type { AssetSnapshot } from "@/api/types";
import type { DetailDescriptor, HistorySectionSpec } from "@/components/detail/types";

type Props<TEntity, TCtx> = {
  descriptor: DetailDescriptor<TEntity, TCtx>;
  assetId: string;
  onBack: () => void;
};

const PAGE_SIZE = 12;

// The five standard tour anchors. Each maps to a core-owned `tour-${step}`
// data-testid and to `${tourKeyPrefix}.tour.${step}Title` / `${step}Body` copy,
// so a step can never point at a region the core doesn't render (ADR-0051).
const TOUR_STEPS = ["overview", "actions", "details", "chart", "snapshots"] as const;

// The generic Position detail screen (ADR-0051). It owns every shared-surface
// concern as hard JSX — back/title/tag control, the actions row
// (help/edit-trigger/terminate/delete + export), the SnapshotChart card + its
// ≥2-snapshot guard, loading/error/not-found, the page scaffold — and touches
// only the Position shared surface (via `descriptor.getAsset`), never a
// `details.*` field. The two variable regions arrive through
// presentation-neutral primitives (`InfoGrid`, `HistorySection`); a descriptor
// supplies only wiring + slots the core calls but never inspects.
export function PositionDetailScreen<TEntity, TCtx>({
  descriptor,
  assetId,
  onBack,
}: Props<TEntity, TCtx>) {
  const { t } = useTranslation(descriptor.i18nNamespaces);
  const { data: entity, isPending, error } = descriptor.useEntity(assetId);
  const { data: snapshots } = useSnapshots(assetId);
  const deleteMutation = descriptor.useDelete();
  const createSnapshotMutation = useCreateSnapshot(assetId);
  const updateSnapshotMutation = useUpdateSnapshot(assetId);
  const deleteSnapshotMutation = useDeleteSnapshot(assetId);
  const importSnapshotMutation = useImportSnapshots(assetId);
  const { data: members } = useHouseholdMembers();
  const { data: currentUser } = useSession();
  const ctx = (descriptor.useDetailContext?.() ?? undefined) as TCtx;

  const [editOpen, setEditOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);

  const { keys } = descriptor;

  function handleConfirmDelete() {
    deleteMutation.mutate(assetId, {
      onSuccess: () => {
        setDeleteOpen(false);
        onBack();
      },
    });
  }

  if (isPending) {
    return <p className="text-sm text-muted-foreground">{t("common:loading")}</p>;
  }
  if (error) {
    return (
      <p className="text-sm text-destructive">
        {t("errors:failedToLoad", { message: (error as Error).message })}
      </p>
    );
  }
  if (!entity) return null;

  const asset = descriptor.getAsset(entity);
  const active = isActiveStatus(asset.status);
  const ownerLabel = ownershipLabel(
    asset.ownership_type,
    asset.sole_owner_user_id,
    members,
    currentUser,
  );

  const headerSecondary = descriptor.headerSecondary(entity, ctx, t);
  const infoFields = descriptor.infoFields(entity, ctx, t);
  const headline = descriptor.renderHeadline?.(entity, ctx);
  const extraSections = descriptor.historySections?.(entity, ctx) ?? [];

  const tourSteps: TourStep[] = TOUR_STEPS.map((step) => ({
    element: `[data-testid="tour-${step}"]`,
    title: t(`${descriptor.tourKeyPrefix}.tour.${step}Title`),
    description: t(`${descriptor.tourKeyPrefix}.tour.${step}Body`),
  }));

  // The universal snapshot history section: core-owned data + mutations, the
  // descriptor supplying only the per-shape header/row/create markup.
  const snapshotSection: HistorySectionSpec<AssetSnapshot> = {
    testId: "tour-snapshots",
    title: t(keys.snapshotsTitle),
    description: t(keys.snapshotsDescription),
    emptyText: t(keys.snapshotsEmpty),
    header: descriptor.snapshot.renderHeader(t),
    rows: snapshots ?? [],
    renderRow: (snapshot) =>
      descriptor.snapshot.renderRow(snapshot, {
        updateMutation: updateSnapshotMutation,
        deleteMutation: deleteSnapshotMutation,
      }),
    pageSize: PAGE_SIZE,
    headerActions: (
      <>
        {/* Export the full position workbook (Detail + Snapshots). Plain anchor
            download — session cookie rides along same-origin. Available
            regardless of status so a terminated position can still be backed up. */}
        <Button
          asChild
          size="sm"
          variant="outline"
          data-testid={`${descriptor.testIdPrefix}-export`}
        >
          <a href={descriptor.exportUrl(assetId)}>
            <Download className="mr-1 size-4" />
            {t("common:export.trigger")}
          </a>
        </Button>
        {active &&
          descriptor.snapshot.renderCreate({
            snapshots: snapshots ?? [],
            currency: asset.native_currency,
            assetId,
            createMutation: createSnapshotMutation,
            importMutation: importSnapshotMutation,
          })}
      </>
    ),
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between gap-4">
        <div>
          <Button variant="ghost" size="sm" onClick={onBack} className="-ml-2 mb-1">
            {t("common:actions.back")}
          </Button>
          <h1 data-testid="tour-overview" className="text-2xl font-semibold tracking-tight">
            {asset.display_name}
          </h1>
          {headerSecondary && <p className="text-sm text-muted-foreground">{headerSecondary}</p>}
          <DetailTagControl
            group={descriptor.tagGroup}
            positionId={asset.id}
            currentTagId={asset.tag_id}
          />
        </div>
        <div data-testid="tour-actions" className="flex gap-2">
          <HelpTourButton steps={tourSteps} />
          <Button variant="outline" size="sm" onClick={() => setEditOpen(true)}>
            <Pencil className="mr-1 size-4" />
            {t("common:actions.edit")}
          </Button>
          <TerminatePositionDialog
            group={descriptor.group}
            id={asset.id}
            listKey={descriptor.listKey}
            currentStatus={asset.status}
            currentTerminatedAt={asset.terminated_at}
            currentNote={asset.termination_note}
          />
          <Button variant="outline" size="sm" onClick={() => setDeleteOpen(true)}>
            <Trash2 className="mr-1 size-4" />
            {t("common:delete")}
          </Button>
        </div>
      </div>

      {headline}

      <Card data-testid="tour-details">
        <CardHeader>
          <CardTitle>{t(keys.detailsCardTitle)}</CardTitle>
          <CardDescription>
            {t(keys.detailsCardLine, {
              ownership: ownerLabel,
              currency: asset.native_currency,
            })}{" "}
            <StatusBadge group={descriptor.group} status={asset.status} />
          </CardDescription>
        </CardHeader>
        {(infoFields.length > 0 || asset.description) && (
          <CardContent className={infoFields.length > 0 ? "space-y-3" : undefined}>
            <InfoGrid fields={infoFields} />
            {asset.description && <p className="text-sm">{asset.description}</p>}
          </CardContent>
        )}
      </Card>

      {snapshots && snapshots.length >= 2 && (
        <Card data-testid="tour-chart">
          <CardHeader>
            <CardTitle>{t(keys.chartTitle)}</CardTitle>
            <CardDescription>
              {t(keys.chartDescription, { currency: asset.native_currency })}
            </CardDescription>
          </CardHeader>
          <CardContent>
            <SnapshotChart snapshots={snapshots} currency={asset.native_currency} />
          </CardContent>
        </Card>
      )}

      <HistorySection {...snapshotSection} />

      {extraSections.map((section) => (
        <HistorySection key={section.testId} {...section} />
      ))}

      {descriptor.renderEditDialog(entity, { open: editOpen, onOpenChange: setEditOpen })}
      <ConfirmDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        title={t(keys.deleteTitle)}
        description={t(keys.deleteDescription)}
        confirmLabel={t("common:delete")}
        destructive
        pending={deleteMutation.isPending}
        onConfirm={handleConfirmDelete}
      />
    </div>
  );
}
