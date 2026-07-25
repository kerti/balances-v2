import type { ReactNode } from "react";
import type { TFunction } from "i18next";
import type { UseMutationResult } from "@tanstack/react-query";
import type { AssetSnapshot } from "@/api/types";
import type { TagGroup } from "@/api/types";
import type { LifecycleGroup } from "@/lib/lifecycle";
import type { TourStep } from "@/components/shell/HelpTourButton";

// The **Position shared surface** the core operates on (ADR-0051, restated from
// CONTEXT.md's Position supertype). Every group — Asset, Investment, Liability,
// Receivable — carries these fields uniformly, so the core reads only this
// projection and never a `details.*` field. `status` widens to `string` because
// each group's status union differs (Asset "closed"/"disposed" vs Investment
// "sold"/"matured") and every consumer (StatusBadge, isActiveStatus, Terminate,
// SnapshotChart) already takes it as a string.
export type Position = {
  id: string;
  display_name: string;
  description: string | null;
  ownership_type: "sole" | "joint";
  sole_owner_user_id: string | null;
  native_currency: string;
  tag_id: string | null;
  status: string;
  terminated_at: string | null;
  termination_note: string | null;
};

// One label/value pair fed to the shared `InfoGrid` (ADR-0051). `value` is
// **already-formatted** content — a string or a neutral node — never a raw
// domain field. Alignment/formatting ride on the value node itself
// (`ml-auto`, `text-right`, `formatCurrency` applied before hand-off); there is
// deliberately no `{ align }` / `{ kind }` flag the grid switches on, which
// would be the god-config slide the ADR bans.
export type InfoField = {
  label: string;
  value: ReactNode;
};

// The minimal slice of a react-query detail result the core consumes. The core
// only ever reaches the Position shared surface via `getAsset`; the rest of the
// entity (its `details.*`) is opaque and reaches the page through slots.
export type DetailQuery<T> = {
  data: T | undefined;
  isPending: boolean;
  error: unknown;
};

export type PositionDeleteMutation = UseMutationResult<unknown, unknown, string>;

// The minimal snapshot shape the *core* touches — the two fields it forwards to
// the `SnapshotChart` and the ≥2 guard. A descriptor's `TSnap` is its concrete
// snapshot type (`AssetSnapshot` for amount-only, `InvestmentSnapshot` for the
// qty×price / accrued investment families); it must satisfy this base so the
// core stays column-blind while descriptor slots see the concrete type.
export type SnapshotShape = {
  year_month: string;
  amount: string;
};

// What a descriptor's snapshot hook hands back (ADR-0051, A3). The descriptor —
// not the core — fetches its own snapshot stream and **binds its create/update/
// delete/import mutations into these closures**, so the two divergent snapshot
// hook families (asset vs investment) never cross the type boundary and the core
// stays free of mutation-variance. The core supplies only `assetId` (known at the
// top of the component, before the entity loads) and, later, `currency`.
export type SnapshotSectionRender<TSnap extends SnapshotShape> = {
  // The snapshot stream — also the core's source for the chart + ≥2 guard and
  // for the investment slots (`renderHeadline` / `chartCostSeries` /
  // `historySections`), which receive it typed as the concrete `TSnap`.
  snapshots: TSnap[] | undefined;
  // Neutral `<TableRow>` of `<TableHead>`s; the primitive never inspects columns.
  header: ReactNode;
  // Neutral `<TableRow>`; mutations already bound inside.
  renderRow: (snapshot: TSnap) => ReactNode;
  // The mobile card variant of `renderRow` (Phase B). `HistorySection` picks it
  // below 768px; optional so a snapshot shape not yet carded falls back to the
  // table on mobile instead of rendering nothing. Mutations already bound inside.
  renderCard?: (snapshot: TSnap) => ReactNode;
  // The create + import controls, mutations bound. The core owns the active-gate
  // (a terminated position hides them) and injects `currency` at render time.
  renderCreateControls: (currency: string) => ReactNode;
};

export type SnapshotSection<TSnap extends SnapshotShape> = {
  // A hook — called once, top-level, by the core (name-prefixed `use*` so the
  // rules-of-hooks lint recognises it). Only needs `assetId`, which is available
  // before the entity resolves.
  useSectionRender: (assetId: string) => SnapshotSectionRender<TSnap>;
};

// One history table — snapshots or (for investments) a transaction ledger. The
// `HistorySection` primitive owns the card + table + pagination and never
// inspects columns; `header` is the neutral `<TableRow>` of `<TableHead>`s and
// `renderRow` returns a neutral `<TableRow>`. `key`ing is the renderer's job.
// `toolbar` (e.g. a transaction search box) and `banner` (e.g. a reconcile
// warning) are optional neutral nodes the primitive renders above the table but
// never reads — the descriptor owns their state/behaviour, keeping the primitive
// presentation-neutral (ADR-0051).
export type HistorySectionSpec<TRow = unknown> = {
  testId: string;
  title: ReactNode;
  description?: ReactNode;
  headerActions?: ReactNode;
  toolbar?: ReactNode;
  banner?: ReactNode;
  emptyText: ReactNode;
  header: ReactNode;
  rows: TRow[];
  renderRow: (row: TRow, index: number) => ReactNode;
  // The mobile card variant (Phase B). When present the primitive renders a
  // stacked card list below 768px instead of the wide table; when absent it
  // stays on the table at every width (a row shape not yet carded).
  renderCard?: (row: TRow, index: number) => ReactNode;
  pageSize: number;
};

// Erase a concretely-typed section to the heterogeneous list the core renders.
// `HistorySectionSpec` is invariant in `TRow` (its `rows` and `renderRow` pull in
// opposite directions), so a typed section can't widen to `HistorySectionSpec`
// directly — but the core only ever pairs a section's `rows` with its own
// `renderRow` inside one `HistorySection`, re-inferring the row type at that call
// site, so the erasure is sound. A descriptor's `historySections` funnels each
// typed section through this at the boundary.
export function erasedSection<T>(section: HistorySectionSpec<T>): HistorySectionSpec {
  return section as unknown as HistorySectionSpec;
}

// The whole spec for one position type's detail page (ADR-0051). Carries wiring
// + slots only; the core owns every Position-shared-surface concern as hard JSX,
// so this never grows a slot that reads a `details.*` field back into the core.
// `TSnap` is the concrete snapshot type (defaults to the amount-only
// `AssetSnapshot`); `TCtx` is the per-render context (investment transactions +
// their mutations + search state), opaque to the core.
export type DetailDescriptor<TEntity, TCtx = void, TSnap extends SnapshotShape = AssetSnapshot> = {
  // Stable identity: `${testIdPrefix}-export` and friends; row keys.
  entityKey: string;
  testIdPrefix: string;
  // `group` drives StatusBadge/Terminate/lifecycle; `tagGroup` is the singular
  // form DetailTagControl wants ("asset" vs "assets").
  group: LifecycleGroup;
  tagGroup: TagGroup;
  // The parent list cache key TerminatePositionDialog invalidates.
  listKey: string;
  i18nNamespaces: string[];

  // Copy the core resolves with its own `t`. Fully-qualified keys, mirroring the
  // list-screen descriptor (ADR-0043). `tourKeyPrefix` builds the five standard
  // tour steps: `${tourKeyPrefix}.tour.${step}Title` / `${step}Body`, each
  // pointed at a core-owned `tour-${step}` anchor so a step can never reference a
  // region the core doesn't render. A type with extra regions (an investment's
  // headline + transaction table) overrides the whole list via `tourSteps`.
  keys: {
    detailsCardTitle: string;
    detailsCardLine: string; // interpolated { ownership, currency }
    chartTitle: string;
    chartDescription: string; // interpolated { currency }
    snapshotsTitle: string;
    snapshotsDescription: string;
    snapshotsEmpty: string;
    deleteTitle: string;
    deleteDescription: string;
  };
  tourKeyPrefix: string;
  // Optional full override of the guided-tour steps, for types whose regions
  // exceed the five standard anchors. Every step still points at an anchor the
  // core or a populated slot renders (`investment-headline`, `tour-transactions`).
  tourSteps?: (t: TFunction) => TourStep[];

  // Data wiring.
  useEntity: (id: string) => DetailQuery<TEntity>;
  useDelete: () => PositionDeleteMutation;
  // Per-render context shared by the slots (e.g. transactions + cost-basis
  // inputs + search state). A hook called once by the core, top-level, with the
  // position id — like the list screen's `useExtraContext`, plus the id the
  // investment families need to fetch their ledgers.
  useDetailContext?: (assetId: string) => TCtx;
  exportUrl: (id: string) => string;

  // Shared-surface projection: the Position fields the core reads off the entity
  // (an Asset for amount-only types, an Investment for the investment families).
  getAsset: (entity: TEntity) => Position;

  // Slots the core calls but never inspects.
  // The grey subtitle under the H1. Optional (ADR-0051 Phase B): the amount-only
  // types moved their identity fields into the details card, so they omit it;
  // the investment families still carry a headline subtitle until their slices.
  headerSecondary?: (entity: TEntity, ctx: TCtx, t: TFunction) => ReactNode;
  // Optional tight, label-less identity block rendered at the top of the details
  // card's left column (ADR-0051 Phase B) — for a type whose identity reads
  // better as a compact cluster (bank name / number / type) than as labelled
  // rows, saving vertical space on mobile. Types without it put their identity in
  // `infoFields` as normal labelled rows; the core never inspects the node.
  identityCluster?: (entity: TEntity, ctx: TCtx, t: TFunction) => ReactNode;
  infoFields: (entity: TEntity, ctx: TCtx, t: TFunction) => InfoField[];
  // Optional neutral surfaces the core drops at fixed page positions but never
  // inspects (ADR-0051, A5 — the outlier tail). `renderBeforeDetails` sits
  // between the header block and the details card; `renderAfterDetails` between
  // the details card and the chart. TimeDeposit — the lone type whose regions
  // exceed the shared skeleton — uses them for its maturity-rollover callout and
  // its rollover-chain card; every other type omits both. Like `renderHeadline`
  // they are `ReactNode` the core renders verbatim, never a field it reads, so the
  // per-type rollover linkage stays in a slot rather than leaking into the core.
  renderBeforeDetails?: (entity: TEntity, ctx: TCtx, t: TFunction) => ReactNode;
  renderAfterDetails?: (entity: TEntity, ctx: TCtx, t: TFunction) => ReactNode;
  // Optional investment headline (has-transactions axis); absent for amount-only.
  // Receives the snapshot stream (for latest value) alongside the ctx (for the
  // cost-basis inputs) — both computed as descriptor wiring, never in the core.
  renderHeadline?: (entity: TEntity, ctx: TCtx, snapshots: TSnap[] | undefined) => ReactNode;
  // The universal snapshot table's per-shape wiring (S1/S2/S3), fetching its own
  // stream and binding its own mutations.
  snapshot: SnapshotSection<TSnap>;
  // Extra history tables beyond snapshots (investment ledgers). Absent for
  // amount-only types; rendered through the same `HistorySection` primitive.
  // Receives the snapshot stream for cross-checks (e.g. quantity reconciliation)
  // and `t` for its copy (titles/headers/empty/toolbar), like the other slots.
  historySections?: (
    entity: TEntity,
    ctx: TCtx,
    snapshots: TSnap[] | undefined,
    t: TFunction,
  ) => HistorySectionSpec[];
  // Optional cost-basis overlay for the `SnapshotChart` (investment-only). The
  // core mounts the chart and forwards this series verbatim; the descriptor
  // computes it from snapshots + its ledger.
  chartCostSeries?: (
    entity: TEntity,
    ctx: TCtx,
    snapshots: TSnap[] | undefined,
  ) => Array<{ year_month: string; cost: number }> | undefined;
  renderEditDialog: (
    entity: TEntity,
    props: { open: boolean; onOpenChange: (open: boolean) => void },
  ) => ReactNode;
};
