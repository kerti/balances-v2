// The non-investment cluster preset (ADR-0043). Bank accounts, properties,
// vehicles, liabilities and receivables all share the same list shape over the
// Position shared surface: an ownership column, and a per-currency
// active-total headline (`activeCurrencyTotals` → `ListHeadline`). This factory
// fills those in so each concrete type is reduced to wiring + copy + its
// group-specific secondary line and dialogs. The investment cluster (#332) is
// the sibling preset with a different headline + a risk filter.
/* eslint-disable react-refresh/only-export-components */
import { useMemo } from "react";
import type { ReactNode } from "react";
import type { TFunction } from "i18next";
import { useTranslation } from "react-i18next";
import { ListHeadline } from "@/components/common/ListHeadline";
import {
  ownershipColumn,
  useOwnershipContext,
  type OwnershipCore,
  type OwnershipCtx,
} from "@/components/positionList/presets/ownership";
import { activeCurrencyTotals } from "@/lib/totals";
import type { LifecycleGroup } from "@/lib/lifecycle";
import type {
  PositionImportMutation,
  PositionListDescriptor,
  PositionListQuery,
  PositionDeleteMutation,
  PositionSnapshotView,
} from "@/components/positionList/types";

// The Position shared-surface fields the preset reads off any list item,
// however the item nests them (`item.asset`, `item.liability`, …).
type PositionCore = OwnershipCore & {
  id: string;
  display_name: string;
  status: string;
};

export type { OwnershipCtx };

export type NonInvestmentConfig<T> = {
  entityKey: string;
  testIdPrefix: string;
  group: LifecycleGroup;
  i18nNamespaces: string[];
  keys: PositionListDescriptor<T>["keys"];
  copyArgs?: (t: TFunction) => Record<string, unknown>;
  useList: () => PositionListQuery<T>;
  useDelete: () => PositionDeleteMutation;
  useImport?: () => PositionImportMutation;
  // Projects a list item down to the shared surface + the snapshot.
  entity: (item: T) => PositionCore;
  getSnapshot: (item: T) => PositionSnapshotView | null;
  getSecondary: (item: T, t: TFunction) => ReactNode;
  // Headline copy + an optional extra block rendered beneath it (the
  // receivables value-over-time chart).
  headlineLabelKey: string;
  headlineTestId: string;
  renderHeadlineExtra?: (items: T[]) => ReactNode;
  renderCreateDialog: (onCreated: (id: string) => void) => ReactNode;
  renderEditDialog: PositionListDescriptor<T>["renderEditDialog"];
};

function NonInvestmentHeadline<T>({
  items,
  config,
}: {
  items: T[];
  config: NonInvestmentConfig<T>;
}) {
  const { t } = useTranslation(config.i18nNamespaces);
  const { totals, count } = useMemo(
    () =>
      activeCurrencyTotals(
        items.map((item) => ({
          status: config.entity(item).status,
          snapshot: config.getSnapshot(item),
        })),
      ),
    [items, config],
  );
  return (
    <>
      <ListHeadline
        totals={totals}
        count={count}
        label={t(config.headlineLabelKey)}
        noun={t(config.keys.noun)}
        nounPlural={t(config.keys.nounPlural)}
        testId={config.headlineTestId}
      />
      {config.renderHeadlineExtra?.(items)}
    </>
  );
}

export function nonInvestmentDescriptor<T>(
  config: NonInvestmentConfig<T>,
): PositionListDescriptor<T, OwnershipCtx> {
  return {
    entityKey: config.entityKey,
    testIdPrefix: config.testIdPrefix,
    group: config.group,
    i18nNamespaces: config.i18nNamespaces,
    defaultSortKey: "name",
    keys: config.keys,
    copyArgs: config.copyArgs,
    useList: config.useList,
    useDelete: config.useDelete,
    useImport: config.useImport,
    useExtraContext: useOwnershipContext,
    getId: (item) => config.entity(item).id,
    getName: (item) => config.entity(item).display_name,
    getStatus: (item) => config.entity(item).status,
    getSnapshot: config.getSnapshot,
    getSecondary: config.getSecondary,
    extraColumns: [ownershipColumn(config.entity)],
    renderHeadline: (items) => <NonInvestmentHeadline items={items} config={config} />,
    renderCreateDialog: config.renderCreateDialog,
    renderEditDialog: config.renderEditDialog,
  };
}
