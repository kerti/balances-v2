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
import { useHouseholdMembers } from "@/hooks/useHouseholdMembers";
import { useSession } from "@/hooks/useSession";
import { isActiveStatus } from "@/lib/lifecycle";
import { ownershipLabel } from "@/lib/ownership";
import { headlineSurface } from "@/lib/headline";
import type {
  DetailDescriptor,
  HistorySectionSpec,
  InfoField,
  SnapshotShape,
} from "@/components/detail/types";

type Props<TEntity, TCtx, TSnap extends SnapshotShape> = {
  descriptor: DetailDescriptor<TEntity, TCtx, TSnap>;
  assetId: string;
  onBack: () => void;
};

const PAGE_SIZE = 12;

// The five standard tour anchors. Each maps to a core-owned `tour-${step}`
// data-testid and to `${tourKeyPrefix}.tour.${step}Title` / `${step}Body` copy,
// so a step can never point at a region the core doesn't render (ADR-0051). A
// type with extra regions overrides the whole list via `descriptor.tourSteps`.
const TOUR_STEPS = ["overview", "actions", "details", "chart", "snapshots"] as const;

// The generic Position detail screen (ADR-0051). It owns every shared-surface
// concern as hard JSX — back/title/tag control, the actions row
// (help/edit-trigger/terminate/delete + export), the SnapshotChart card + its
// ≥2-snapshot guard, loading/error/not-found, the page scaffold — and touches
// only the Position shared surface (via `descriptor.getAsset`), never a
// `details.*` field. The two variable regions arrive through
// presentation-neutral primitives (`InfoGrid`, `HistorySection`); a descriptor
// supplies only wiring + slots the core calls but never inspects. The snapshot
// stream + its mutations are the descriptor's own (asset vs investment families
// diverge), bound into `renderRow`/`renderCreateControls` closures so no mutation
// type crosses the boundary.
export function PositionDetailScreen<TEntity, TCtx, TSnap extends SnapshotShape>({
  descriptor,
  assetId,
  onBack,
}: Props<TEntity, TCtx, TSnap>) {
  const { t } = useTranslation(descriptor.i18nNamespaces);
  const { data: entity, isPending, error } = descriptor.useEntity(assetId);
  const snapshotRender = descriptor.snapshot.useSectionRender(assetId);
  const deleteMutation = descriptor.useDelete();
  const { data: members } = useHouseholdMembers();
  const { data: currentUser } = useSession();
  const ctx = (descriptor.useDetailContext?.(assetId) ?? undefined) as TCtx;

  const [editOpen, setEditOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);

  const { keys } = descriptor;
  const snapshots = snapshotRender.snapshots;

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

  const headerSecondary = descriptor.headerSecondary?.(entity, ctx, t);
  const infoFields = descriptor.infoFields(entity, ctx, t);
  const identity = descriptor.identityCluster?.(entity, ctx, t);
  // The details card is a two-column headline on desktop (ADR-0051 Phase B, the
  // Income headline idiom): the descriptor's own identity + spec fields fill the
  // left column, the shared-surface meta the right; both collapse to one stacked
  // column on mobile. Currency + status ride the card title instead, to save the
  // vertical space extra rows would cost — leaving ownership the sole meta row.
  const metaFields: InfoField[] = [{ label: t("common:fields.ownership"), value: ownerLabel }];
  const headline = descriptor.renderHeadline?.(entity, ctx, snapshots);
  const beforeDetails = descriptor.renderBeforeDetails?.(entity, ctx, t);
  const afterDetails = descriptor.renderAfterDetails?.(entity, ctx, t);
  const extraSections = descriptor.historySections?.(entity, ctx, snapshots, t) ?? [];
  const costSeries = descriptor.chartCostSeries?.(entity, ctx, snapshots);

  const tourSteps: TourStep[] =
    descriptor.tourSteps?.(t) ??
    TOUR_STEPS.map((step) => ({
      element: `[data-testid="tour-${step}"]`,
      title: t(`${descriptor.tourKeyPrefix}.tour.${step}Title`),
      description: t(`${descriptor.tourKeyPrefix}.tour.${step}Body`),
    }));

  // The universal snapshot history section: the descriptor owns the data + row +
  // create markup (mutations bound inside), the core owns the card frame, the
  // title/description/empty copy, the export button and the active-gate.
  const snapshotSection: HistorySectionSpec<TSnap> = {
    testId: "tour-snapshots",
    title: t(keys.snapshotsTitle),
    description: t(keys.snapshotsDescription),
    emptyText: t(keys.snapshotsEmpty),
    header: snapshotRender.header,
    rows: snapshots ?? [],
    renderRow: (snapshot) => snapshotRender.renderRow(snapshot),
    // Forward the shape's mobile card renderer only when it exists, so
    // `HistorySection` stays on the table for shapes not yet carded (Phase B).
    ...(snapshotRender.renderCard && {
      renderCard: (snapshot: TSnap) => snapshotRender.renderCard!(snapshot),
    }),
    pageSize: PAGE_SIZE,
    headerActions: (
      <>
        {/* Export the full position workbook (Detail + Snapshots [+ Transactions
            for investments]). Plain anchor download — session cookie rides along
            same-origin. Available regardless of status so a terminated position
            can still be backed up. */}
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
        {active && snapshotRender.renderCreateControls(asset.native_currency)}
      </>
    ),
  };

  return (
    <div className="space-y-6">
      <div className="flex flex-col items-start gap-4 md:flex-row md:items-center md:justify-between">
        <div>
          <Button variant="ghost" size="sm" onClick={onBack} className="-ml-2 mb-1">
            {t("common:actions.back")}
          </Button>
          <h1 data-testid="tour-overview" className="text-2xl font-semibold tracking-tight">
            {asset.display_name}
          </h1>
          {headerSecondary && <p className="text-sm text-muted-foreground">{headerSecondary}</p>}
          {headline}
        </div>
        <div data-testid="tour-actions" className="flex flex-wrap gap-2">
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

      {beforeDetails}

      <Card data-testid="tour-details" className={headlineSurface}>
        <CardHeader>
          <div className="flex items-center justify-between gap-3">
            <CardTitle>{t(keys.detailsCardTitle)}</CardTitle>
            {/* Currency + status ride the title line, right-aligned, to save the
                vertical space two more meta rows would cost (ADR-0051 Phase B). */}
            <div className="flex items-center gap-2 text-sm">
              <span className="tabular-nums text-muted-foreground">{asset.native_currency}</span>
              <StatusBadge group={descriptor.group} status={asset.status} />
            </div>
          </div>
        </CardHeader>
        <CardContent className="space-y-3">
          {/* Two columns on desktop with a divider between (the Income headline
              layout) — identity + spec + tag on the left, the shared meta on the
              right; one stacked column on mobile. */}
          <div className="grid gap-x-6 gap-y-4 md:grid-cols-2 md:gap-y-0 md:divide-x md:divide-border md:[&>*]:px-6 md:[&>*:first-child]:pl-0 md:[&>*:last-child]:pr-0">
            <div className="space-y-3">
              {identity}
              <InfoGrid fields={infoFields} />
            </div>
            <div className="space-y-3">
              <InfoGrid fields={metaFields} mobileLayout="inline" />
              <DetailTagControl
                group={descriptor.tagGroup}
                positionId={asset.id}
                currentTagId={asset.tag_id}
              />
            </div>
          </div>
          {asset.description && <p className="text-sm">{asset.description}</p>}
        </CardContent>
      </Card>

      {afterDetails}

      {snapshots && snapshots.length >= 2 && (
        <Card data-testid="tour-chart">
          <CardHeader>
            <CardTitle>{t(keys.chartTitle)}</CardTitle>
            <CardDescription>
              {t(keys.chartDescription, { currency: asset.native_currency })}
            </CardDescription>
          </CardHeader>
          <CardContent>
            <SnapshotChart
              snapshots={snapshots}
              currency={asset.native_currency}
              costSeries={costSeries}
              status={asset.status}
            />
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
