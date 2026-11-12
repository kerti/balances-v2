import { useState } from "react";
import { useSearchParams } from "react-router";
import { PageHeader } from "@/components/common/PageHeader";
import { Download, Pencil } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { TerminatePositionDialog } from "@/components/dialogs/TerminatePositionDialog";
import { ConfirmDialog } from "@/components/dialogs/ConfirmDialog";
import { StatusBadge } from "@/components/common/StatusBadge";
import { SnapshotChart } from "@/components/charts/SnapshotChart";
import { HelpTourButton, type TourStep } from "@/components/shell/HelpTourButton";
import { InfoGrid } from "@/components/detail/InfoGrid";
import { HistorySection } from "@/components/detail/HistorySection";
import { PositionActionsMenu } from "@/components/detail/PositionActionsMenu";
import { useHouseholdMembers } from "@/hooks/useHouseholdMembers";
import { useSession } from "@/hooks/useSession";
import { isActiveStatus } from "@/lib/lifecycle";
import { ownershipLabel } from "@/lib/ownership";
import { headlineSurface } from "@/lib/headline";
import { RecordSnapshotRequestProvider } from "@/components/detail/RecordSnapshotRequestProvider";
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

// The five standard tour anchors. Each maps to a core-owned `tour-${step}`
// data-testid and to `${tourKeyPrefix}.tour.${step}Title` / `${step}Body` copy,
// so a step can never point at a region the core doesn't render (ADR-0051). A
// type with extra regions overrides the whole list via `descriptor.tourSteps`.
const TOUR_STEPS = ["overview", "actions", "details", "chart", "snapshots"] as const;

// The two history tabs. `?tab=` carries the open one so a link, a refresh and
// Back all land on it (#839); Snapshots is the default and leaves no param.
type HistoryTab = "snapshots" | "transactions";
const TAB_PARAM = "tab";

// The generic Position detail screen (ADR-0051). It owns every shared-surface
// concern as hard JSX — back/title, the actions row (help · edit · ⋯ menu with
// close/change-status and delete), the details card, the SnapshotChart card + its
// ≥2-snapshot guard, the snapshots/transactions tabs, loading/error/not-found,
// the page scaffold — and touches only the Position shared surface (via
// `descriptor.getAsset`), never a `details.*` field. The variable regions arrive
// through presentation-neutral primitives (`InfoGrid`, `HistorySection`); a
// descriptor supplies only wiring + slots the core calls but never inspects. The
// snapshot stream + its mutations are the descriptor's own (asset vs investment
// families diverge), bound into `renderRow`/`renderCreateControls` closures so no
// mutation type crosses the boundary.
export function PositionDetailScreen<TEntity, TCtx, TSnap extends SnapshotShape>(
  props: Props<TEntity, TCtx, TSnap>,
) {
  // The provider sits above the body so the descriptor hooks it calls (a trade's
  // toast) and the Record snapshot dialog it renders share one request channel
  // (#830).
  return (
    <RecordSnapshotRequestProvider>
      <PositionDetailBody {...props} />
    </RecordSnapshotRequestProvider>
  );
}

function PositionDetailBody<TEntity, TCtx, TSnap extends SnapshotShape>({
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
  const [searchParams, setSearchParams] = useSearchParams();

  const [editOpen, setEditOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [closeOpen, setCloseOpen] = useState(false);

  const { keys } = descriptor;
  const snapshots = snapshotRender.snapshots;

  function handleConfirmDelete() {
    if (!entity) return;
    const name = descriptor.getAsset(entity).display_name;
    deleteMutation.mutate(
      { id: assetId, name },
      {
        onSuccess: () => {
          setDeleteOpen(false);
          onBack();
        },
      },
    );
  }

  function selectTab(next: HistoryTab) {
    // Replace, not push: flipping tabs is not a navigation, and Back should leave
    // the page rather than step through the tabs.
    setSearchParams(
      (prev) => {
        const params = new URLSearchParams(prev);
        if (next === "snapshots") params.delete(TAB_PARAM);
        else params.set(TAB_PARAM, next);
        return params;
      },
      { replace: true },
    );
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
  const headerBadge = descriptor.renderHeaderBadge?.(entity, ctx, t);
  const identity = descriptor.identityCluster?.(entity, ctx, t);
  // The details card is one identity line plus one labelled grid (#839): the
  // type's own fields, then the investment money summary, then the shared
  // ownership row. Currency + status ride the card title instead, to save the
  // vertical space extra rows would cost.
  const headlineFields = descriptor.headlineFields?.(entity, ctx, snapshots, t);
  const metaFields: InfoField[] = [{ label: t("common:fields.ownership"), value: ownerLabel }];
  const gridFields = [
    ...descriptor.infoFields(entity, ctx, t),
    ...(headlineFields ?? []),
    ...metaFields,
  ];
  const beforeDetails = descriptor.renderBeforeDetails?.(entity, ctx, t);
  const afterDetails = descriptor.renderAfterDetails?.(entity, ctx, t);
  const extraSections = descriptor.historySections?.(entity, ctx, snapshots, t) ?? [];
  const costSeries = descriptor.chartCostSeries?.(entity, ctx, snapshots);

  // A tab bar only when the position has a ledger to tab to (an account has none).
  const transactionsSection = extraSections[0];
  const requestedTab = searchParams.get(TAB_PARAM);
  const tab: HistoryTab =
    transactionsSection && requestedTab === "transactions" ? "transactions" : "snapshots";

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
    // Two deliberate rows (#542), stacked on phones and inline from 768px up:
    // row 1 = the single Record snapshot button (#788), the promoted primary,
    // row 2 = the workbook *I/O* pair (Export + Import). Export is always
    // present so a terminated position can still be backed up; the create row
    // and Import hide when the position is closed. On phones the header goes
    // full width and each row's buttons split it evenly (`max-md:[&>*]:flex-1`,
    // so Record snapshot fills row 1); from 768px up they fall back to natural,
    // inline widths.
    headerActions: (
      <div className="flex w-full flex-col gap-2 md:w-auto md:flex-row md:flex-wrap md:items-center">
        {active && (
          <div className="flex gap-2 max-md:[&>*]:flex-1" data-testid="snapshot-create-row">
            {snapshotRender.renderCreateControls(asset.native_currency)}
          </div>
        )}
        <div className="flex gap-2 max-md:[&>*]:flex-1" data-testid="snapshot-io-row">
          {/* Export the full position workbook (Detail + Snapshots [+ Transactions
              for investments]). Plain anchor download — session cookie rides along
              same-origin. */}
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
          {active && snapshotRender.renderImportControl(asset.native_currency)}
        </div>
      </div>
    ),
  };

  return (
    <div className="space-y-6">
      <PageHeader
        variant="sub-page"
        back={{ label: t("common:actions.back"), onClick: onBack }}
        title={asset.display_name}
        titleTestId="tour-overview"
        subtitle={headerSecondary}
        // One row at every width (#839): `?` Help, Edit (the header's one primary
        // button; it takes the spare width on phones) and the ⋯ menu holding
        // Close… / Change status and the red Delete.
        actions={
          <div
            data-testid="tour-actions"
            className="flex w-full items-center gap-2 md:w-auto md:justify-end"
          >
            <HelpTourButton steps={tourSteps} />
            <Button className="max-md:flex-1" onClick={() => setEditOpen(true)}>
              <Pencil className="mr-1 size-4" />
              {t("common:actions.edit")}
            </Button>
            <PositionActionsMenu
              active={active}
              onClose={() => setCloseOpen(true)}
              onDelete={() => setDeleteOpen(true)}
            />
          </div>
        }
      />

      {beforeDetails}

      <Card data-testid="tour-details" className={headlineSurface}>
        <CardHeader>
          <div className="flex items-center justify-between gap-3">
            <CardTitle>{t(keys.detailsCardTitle)}</CardTitle>
            {/* Header badge (investment risk shield) + currency + status ride the
                title line, right-aligned, to save the vertical space more meta
                rows would cost (ADR-0051 Phase B). */}
            <div className="flex items-center gap-2 text-sm">
              {headerBadge}
              <span className="text-muted-foreground">{asset.native_currency}</span>
              <StatusBadge group={descriptor.group} status={asset.status} />
            </div>
          </div>
          {identity}
        </CardHeader>
        <CardContent className="space-y-3">
          {/* The anchor of the "Cost and gain / loss" tour step: the grid carries
              the investment money summary. */}
          <div data-testid={headlineFields ? "investment-headline" : undefined}>
            <InfoGrid fields={gridFields} />
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
              primaryLabel={keys.chartSeriesLabel ? t(keys.chartSeriesLabel) : undefined}
              costSeries={costSeries}
              status={asset.status}
            />
          </CardContent>
        </Card>
      )}

      {transactionsSection ? (
        /* Snapshots | Transactions. Both panels stay mounted (`forceMount`, the
           inactive one hidden by CSS) so the Record snapshot dialog a trade's toast
           or the reconcile banner asks for (#830) can open from the Transactions tab,
           and each tab keeps its own page and search state. */
        <Tabs
          value={tab}
          onValueChange={(next) => selectTab(next as HistoryTab)}
          className="flex-col gap-4"
          data-testid="history-tabs"
        >
          <TabsList className="h-9 w-full max-md:h-11 md:w-fit">
            <TabsTrigger
              value="snapshots"
              className={tabTriggerClass}
              data-testid="history-tab-snapshots"
            >
              {t("common:historyTabs.snapshots", { count: snapshots?.length ?? 0 })}
            </TabsTrigger>
            <TabsTrigger
              value="transactions"
              className={tabTriggerClass}
              data-testid="history-tab-transactions"
            >
              {t("common:historyTabs.transactions", {
                count: transactionsSection.totalCount ?? transactionsSection.rows.length,
              })}
            </TabsTrigger>
          </TabsList>
          <TabsContent
            value="snapshots"
            forceMount
            className={panelClass}
            data-testid="history-panel-snapshots"
          >
            <HistorySection {...snapshotSection} />
          </TabsContent>
          <TabsContent
            value="transactions"
            forceMount
            className={panelClass}
            data-testid="history-panel-transactions"
          >
            {extraSections.map((section) => (
              <HistorySection key={section.testId} {...section} />
            ))}
          </TabsContent>
        </Tabs>
      ) : (
        <HistorySection {...snapshotSection} />
      )}

      {descriptor.renderEditDialog(entity, { open: editOpen, onOpenChange: setEditOpen })}
      <TerminatePositionDialog
        group={descriptor.group}
        id={asset.id}
        listKey={descriptor.listKey}
        currentStatus={asset.status}
        currentTerminatedAt={asset.terminated_at}
        currentNote={asset.termination_note}
        investmentSubtype={descriptor.investmentSubtype}
        assetSubtype={descriptor.assetSubtype}
        currency={asset.native_currency}
        open={closeOpen}
        onOpenChange={setCloseOpen}
      />
      <ConfirmDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        title={t("common:deleteConfirm.positionTitle", { name: asset.display_name })}
        description={t("common:deleteConfirm.positionDescription")}
        confirmLabel={t("common:delete")}
        destructive
        pending={deleteMutation.isPending}
        onConfirm={handleConfirmDelete}
      />
    </div>
  );
}

// The tab strip's look. The shadcn TabsTrigger keys its active state off a
// `data-active` attribute Radix never sets, so the active style is spelled out
// against `data-state`; `max-md:min-h-11` is the phone tap floor.
const tabTriggerClass =
  "max-md:min-h-11 data-[state=active]:bg-background data-[state=active]:text-foreground data-[state=active]:shadow-sm";

// Radix leaves a force-mounted inactive panel in the DOM; hide it with CSS.
const panelClass = "data-[state=inactive]:hidden";
