// The non-investment cluster preset (ADR-0043). Bank accounts, properties,
// vehicles, liabilities and receivables all share the same list shape over the
// Position shared surface: an ownership column, and a headline that is the
// list's converted net-worth share (`useListConversion` → `ConvertedHeadline`,
// #937) — or, for a household with nothing outside the reporting currency, the
// per-currency active total (`activeCurrencyTotals` → `ListHeadline`). This factory
// fills those in so each concrete type is reduced to wiring + copy + its
// group-specific secondary line and dialogs. The investment cluster (#332) is
// the sibling preset with a different headline + a risk filter.
/* eslint-disable react-refresh/only-export-components */
import { useMemo } from "react";
import { useSession } from "@/hooks/useSession";
import type { ReactNode } from "react";
import type { TFunction } from "i18next";
import { useTranslation } from "react-i18next";
import { ListHeadline } from "@/components/common/ListHeadline";
import { ConvertedHeadline } from "@/components/common/ConvertedHeadline";
import { MissingFxWarning } from "@/components/common/MissingFxWarning";
import {
  useListConversion,
  type ListBreakdown,
  type ListConversion,
} from "@/hooks/useListConversion";
import {
  ownershipColumn,
  useOwnershipContext,
  type OwnershipCore,
  type OwnershipCtx,
} from "@/components/positionList/presets/ownership";
import { activeCurrencyTotals } from "@/lib/totals";
import type { AssetSubtype, LifecycleGroup } from "@/lib/lifecycle";
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
  native_currency: string;
  terminated_at: string | null;
  termination_note: string | null;
};

export type { OwnershipCtx };

export type NonInvestmentConfig<T> = {
  entityKey: string;
  testIdPrefix: string;
  group: LifecycleGroup;
  // The list cache key the lifecycle dialog invalidates, and (assets only) the
  // subtype that narrows its statuses — same values as the detail descriptor.
  listKey: string;
  assetSubtype?: AssetSubtype;
  // The engine slice the headline shows (whole group or one subtype).
  breakdown: ListBreakdown;
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
  // receivables value-over-time chart), told whether the headline converted.
  headlineLabelKey: string;
  headlineTestId: string;
  renderHeadlineExtra?: (items: T[], conversion: ListConversion) => ReactNode;
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
  const reporting = useSession().data?.reporting_currency ?? "";
  const { totals, count } = useMemo(
    () =>
      activeCurrencyTotals(
        items.map((item) => ({
          status: config.entity(item).status,
          snapshot: config.getSnapshot(item),
        })),
        reporting,
      ),
    [items, config, reporting],
  );
  const conversion = useListConversion(
    config.breakdown,
    items.map((item) => {
      const core = config.entity(item);
      return {
        id: core.id,
        status: core.status,
        currency: config.getSnapshot(item)?.currency ?? "",
      };
    }),
  );
  const label = t(config.headlineLabelKey);
  const noun = t(config.keys.noun);
  const nounPlural = t(config.keys.nounPlural);
  return (
    <>
      {conversion.mode === "converted" ? (
        <ConvertedHeadline
          label={label}
          total={conversion.total}
          currency={conversion.currency}
          nativeSubtotals={totals
            .filter((r) => r.currency !== conversion.currency)
            .map((r) => ({ currency: r.currency, value: r.amount }))}
          countText={t("common:list.activeCount", {
            count,
            noun: count === 1 ? noun : nounPlural,
          })}
          testId={config.headlineTestId}
        >
          <MissingFxWarning
            missingFx={conversion.missingFx}
            yearMonth={conversion.yearMonth}
            scope="groupTotal"
          />
        </ConvertedHeadline>
      ) : (
        conversion.mode === "native" && (
          <ListHeadline
            totals={totals}
            count={count}
            label={label}
            noun={noun}
            nounPlural={nounPlural}
            testId={config.headlineTestId}
          />
        )
      )}
      {config.renderHeadlineExtra?.(items, conversion)}
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
    lifecycle: { listKey: config.listKey, assetSubtype: config.assetSubtype },
    getLifecycle: (item) => {
      const core = config.entity(item);
      return {
        terminatedAt: core.terminated_at,
        note: core.termination_note,
        currency: core.native_currency,
      };
    },
    extraColumns: [ownershipColumn(config.entity)],
    renderHeadline: (items) => <NonInvestmentHeadline items={items} config={config} />,
    renderCreateDialog: config.renderCreateDialog,
    renderEditDialog: config.renderEditDialog,
  };
}
