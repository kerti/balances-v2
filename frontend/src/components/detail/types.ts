import type { ReactNode } from "react";
import type { TFunction } from "i18next";
import type { UseMutationResult } from "@tanstack/react-query";
import type { Asset, AssetSnapshot } from "@/api/types";
import type { TagGroup } from "@/api/types";
import type { LifecycleGroup } from "@/lib/lifecycle";
import type {
  useCreateSnapshot,
  useUpdateSnapshot,
  useDeleteSnapshot,
  useImportSnapshots,
} from "@/hooks/useAssetSnapshots";

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

// The core-owned snapshot mutations handed to a descriptor's snapshot renderers.
// These are shared across every position group (ADR-0022), so the core owns the
// hooks and passes them down — the descriptor only wires them into the right
// per-shape Row + create dialog (S1/S2/S3).
export type SnapshotMutations = {
  updateMutation: ReturnType<typeof useUpdateSnapshot>;
  deleteMutation: ReturnType<typeof useDeleteSnapshot>;
};

export type SnapshotCreateArgs = {
  snapshots: AssetSnapshot[];
  currency: string;
  assetId: string;
  createMutation: ReturnType<typeof useCreateSnapshot>;
  importMutation: ReturnType<typeof useImportSnapshots>;
};

// The variable bits of the universal snapshot history section. The core owns the
// data (its snapshot stream) + the mutations; the descriptor supplies only the
// per-shape header/row/create markup, which the core assembles into a generic
// `HistorySectionSpec`. `renderRow` reuses the existing S1/S2/S3 snapshot
// renderers (`SnapshotRow`, `QuantityPriceSnapshotRow`,
// `AccruedInterestSnapshotRow`) — the primitive never learns which.
export type SnapshotSection = {
  renderHeader: (t: TFunction) => ReactNode;
  renderRow: (snapshot: AssetSnapshot, mutations: SnapshotMutations) => ReactNode;
  renderCreate: (args: SnapshotCreateArgs) => ReactNode;
};

// One history table — snapshots or (for investments) a transaction ledger. The
// `HistorySection` primitive owns the card + table + pagination and never
// inspects columns; `header` is the neutral `<TableRow>` of `<TableHead>`s and
// `renderRow` returns a neutral `<TableRow>`. `key`ing is the renderer's job.
export type HistorySectionSpec<TRow = unknown> = {
  testId: string;
  title: ReactNode;
  description?: ReactNode;
  headerActions?: ReactNode;
  emptyText: ReactNode;
  header: ReactNode;
  rows: TRow[];
  renderRow: (row: TRow, index: number) => ReactNode;
  pageSize: number;
};

// The whole spec for one position type's detail page (ADR-0051). Carries wiring
// + slots only; the core owns every Position-shared-surface concern as hard JSX,
// so this never grows a slot that reads a `details.*` field back into the core.
export type DetailDescriptor<TEntity, TCtx = void> = {
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
  // region the core doesn't render.
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

  // Data wiring.
  useEntity: (id: string) => DetailQuery<TEntity>;
  useDelete: () => PositionDeleteMutation;
  // Per-render context shared by the slots (e.g. cost-basis inputs). Called once
  // by the core, top-level, like the list screen's `useExtraContext`.
  useDetailContext?: () => TCtx;
  exportUrl: (id: string) => string;

  // Shared-surface projection: the one field the core reads off the entity.
  getAsset: (entity: TEntity) => Asset;

  // Slots the core calls but never inspects.
  headerSecondary: (entity: TEntity, ctx: TCtx, t: TFunction) => ReactNode;
  infoFields: (entity: TEntity, ctx: TCtx, t: TFunction) => InfoField[];
  // Optional investment headline (has-transactions axis); absent for amount-only.
  renderHeadline?: (entity: TEntity, ctx: TCtx) => ReactNode;
  snapshot: SnapshotSection;
  // Extra history tables beyond snapshots (investment ledgers). Absent for
  // amount-only types; rendered through the same `HistorySection` primitive.
  historySections?: (entity: TEntity, ctx: TCtx) => HistorySectionSpec[];
  renderEditDialog: (
    entity: TEntity,
    props: { open: boolean; onOpenChange: (open: boolean) => void },
  ) => ReactNode;
};
