import type { ReactNode } from "react";
import type { TFunction } from "i18next";
import type { UseMutationResult } from "@tanstack/react-query";
import type { LifecycleGroup } from "@/lib/lifecycle";
import type { SortDir } from "@/lib/sort";
import type { CreateImportArgs, CreateImportResult } from "@/hooks/snapshotImport";

// The presentation-neutral shape the core reads off a position's latest
// snapshot. A descriptor projects its list item down to this; the core owns how
// it renders (amount + month, or an em dash when null).
export type PositionSnapshotView = {
  amount: string;
  currency: string;
  year_month: string;
};

// The minimal slice of a react-query list result the core consumes.
export type PositionListQuery<T> = {
  data: T[] | undefined;
  isPending: boolean;
  error: unknown;
};

export type PositionDeleteMutation = UseMutationResult<unknown, unknown, string>;
export type PositionImportMutation = UseMutationResult<
  CreateImportResult,
  unknown,
  CreateImportArgs
>;

// A group-specific column beyond the four shared-surface columns. `render`
// returns presentation-neutral value content (string / fragment), never a
// `<TableCell>` — the renderer owns the wrapper (ADR-0043). `sort` is optional;
// when present the core wires a sortable header + client sort. `mobile:
// "secondary"` opts the column into the mobile card (default: hidden on mobile,
// the card shows the shared surface only).
export type PositionExtraColumn<T, Ctx> = {
  id: string;
  labelKey: string;
  align?: "left" | "right";
  // "main" sits between name and status (ownership, activity); "trailing" sits
  // after the value column. Default "main".
  slot?: "main" | "trailing";
  mobile?: "secondary";
  render: (item: T, ctx: Ctx) => ReactNode;
  sort?: {
    key: string;
    type: "text" | "number";
    value: (item: T, ctx: Ctx) => string | number | null;
  };
};

// A row-level filter beyond show-inactive (the investment risk-profile filter
// is the first consumer, #332). The core renders `control` beside the
// show-inactive toggle and keeps only rows the `predicate` admits.
export type PositionRowFilter<T> = {
  control: ReactNode;
  predicate: (item: T) => boolean;
};

// The whole spec for one position type's list screen. Carries wiring + slots
// only; the core owns all shared-surface markup, so this never grows a
// `renderRow` or a shared-column override — that is the guardrail against a
// god-config (ADR-0043).
export type PositionListDescriptor<T, Ctx = void> = {
  // Stable identity: row keys + the list's test ids (`${testIdPrefix}-row`).
  entityKey: string;
  testIdPrefix: string;
  group: LifecycleGroup;
  i18nNamespaces: string[];
  defaultSortKey: string;

  // Copy: i18n keys the core resolves with its own `t`. Interpolated lines
  // (secondary, delete description) are functions instead.
  keys: {
    listTitle: string;
    listSubtitle: string;
    emptyTitle: string;
    emptyBody: string;
    noun: string;
    nounPlural: string;
    valueLabel: string; // header for the shared value column
    rowActions: string; // aria-label on the ⋮ trigger
    deleteTitle: string;
  };
  // Optional interpolation values merged into the title/subtitle/empty-state
  // `t()` calls, for copy that varies by a runtime parameter (e.g. a liability
  // screen's subtype noun). Keys absent from a given string are ignored.
  copyArgs?: (t: TFunction) => Record<string, unknown>;

  // Data wiring.
  useList: () => PositionListQuery<T>;
  useDelete: () => PositionDeleteMutation;
  useImport?: () => PositionImportMutation;
  // Per-render context shared by the extra columns (e.g. household members for
  // the ownership label). Called once by the core.
  useExtraContext?: () => Ctx;
  useRowFilter?: () => PositionRowFilter<T>;

  // Shared-surface projection.
  getId: (item: T) => string;
  getName: (item: T) => string;
  getStatus: (item: T) => string;
  getSnapshot: (item: T) => PositionSnapshotView | null;
  getSecondary: (item: T, t: TFunction) => ReactNode;
  deleteDescription: (item: T, t: TFunction) => string;

  extraColumns: PositionExtraColumn<T, Ctx>[];

  // Slots the core calls but never inspects.
  // An optional inline accessory beside the display name in the title cell
  // (the investment risk-profile badge). Absent for types without one.
  renderTitleAccessory?: (item: T, ctx: Ctx) => ReactNode;
  renderHeadline: (items: T[]) => ReactNode;
  renderCreateDialog: () => ReactNode;
  renderEditDialog: (
    item: T,
    props: { open: boolean; onOpenChange: (open: boolean) => void },
  ) => ReactNode;
};

// ----- internal view models (core → renderers) --------------------------

// One row, already projected off its list item into the shared surface. The
// renderers never touch the raw item except to pass it back through row-action
// callbacks.
export type RowView<T> = {
  item: T;
  id: string;
  name: string;
  secondary: ReactNode;
  status: string;
  statusText: string;
  amount: number | null;
  snapshot: PositionSnapshotView | null;
  terminated: boolean;
};

// One column, resolved by the core to hard markup in `cell`. Both renderers
// iterate the same list; the web renderer wraps each `cell` in a `<TableCell>`,
// the mobile renderer drops the ones without `mobileVisible`.
export type ColumnView<T> = {
  id: string;
  label: string;
  align: "left" | "right";
  sortKey?: string;
  isTitle?: boolean; // the name column; the card heading
  mobileVisible: boolean; // shown on the mobile card
  cell: (row: RowView<T>) => ReactNode;
};

export type PositionListRendererProps<T> = {
  rows: RowView<T>[];
  columns: ColumnView<T>[];
  sortKey: string;
  sortDir: SortDir;
  onToggleSort: (key: string) => void;
  onSelect: (id: string) => void;
  onEdit: (item: T) => void;
  onDelete: (item: T) => void;
  actionsLabel: string;
  testIdPrefix: string;
};
