// The investment cluster preset (ADR-0043). The sibling to the non-investment
// preset: same `PositionListScreen` core, different configuration. Investments
// swap the ownership column for a risk-profile badge beside the name + a
// risk-profile row filter, add a shared transaction-activity column, and use
// the cost/P-L headline (`InvestmentListHeadline` + `ListTimeGraph` over
// `aggregateListPositions`) instead of the single-figure `ListHeadline`. A
// concrete investment type supplies only its subtype columns (e.g. gold's form
// + purity) plus wiring and dialogs.
/* eslint-disable react-refresh/only-export-components */
import { useMemo, useState } from "react";
import type { ReactNode } from "react";
import type { TFunction } from "i18next";
import { useTranslation } from "react-i18next";
import { InvestmentListHeadline } from "@/components/InvestmentListHeadline";
import { ListTimeGraph } from "@/components/ListTimeGraph";
import { RiskProfileBadge } from "@/components/RiskProfileBadge";
import {
  RiskProfileFilter,
  type RiskProfileFilterValue,
} from "@/components/RiskProfileFilter";
import { useInvestmentTimeSeries } from "@/hooks/useInvestmentTimeSeries";
import { aggregateListPositions, type Position } from "@/lib/listAggregates";
import { formatDate } from "@/lib/format";
import type { RiskProfile } from "@/api/types";
import type {
  PositionDeleteMutation,
  PositionExtraColumn,
  PositionImportMutation,
  PositionListDescriptor,
  PositionListQuery,
  PositionSnapshotView,
} from "@/components/positionList/types";

// The shared-surface fields the preset reads off any investment list item,
// plus the two investment-only bits it needs (risk profile, description).
type InvestmentCore = {
  id: string;
  display_name: string;
  status: string;
  native_currency: string;
  terminated_at: string | null;
  risk_profile: RiskProfile;
  description: string | null;
};

export type InvestmentConfig<T> = {
  entityKey: string;
  testIdPrefix: string;
  i18nNamespaces: string[];
  keys: PositionListDescriptor<T>["keys"];
  useList: () => PositionListQuery<T>;
  useDelete: () => PositionDeleteMutation;
  useImport?: () => PositionImportMutation;
  entity: (item: T) => InvestmentCore;
  getSnapshot: (item: T) => PositionSnapshotView | null;
  // Avg-cost basis folded into the list payload (issue #18), as a number.
  costBasis: (item: T) => number;
  // Ledger summary for the shared activity column (issue #67).
  activity: (item: T) => { count: number; lastDate: string | null };
  // Subtype-specific columns between name and status (gold's form+purity).
  mainColumns?: PositionExtraColumn<T, void>[];
  deleteDescription: (item: T, t: TFunction) => string;
  headlineTestId: string;
  renderCreateDialog: () => ReactNode;
  renderEditDialog: PositionListDescriptor<T>["renderEditDialog"];
};

// The transaction-activity cell content (issue #67), lifted out of
// TransactionActivityCell's `<TableCell>` wrapper so it reads as a
// presentation-neutral extra column.
function ActivityContent({
  count,
  lastDate,
}: {
  count: number;
  lastDate: string | null;
}) {
  const { t } = useTranslation("common");
  if (count === 0) return <span className="text-muted-foreground">{"—"}</span>;
  return (
    <>
      <div className="text-sm">{t("common:activity.count", { count })}</div>
      {lastDate && (
        <div className="text-xs text-muted-foreground">
          {t("common:activity.last", { date: formatDate(lastDate) })}
        </div>
      )}
    </>
  );
}

// The cost + unrealized-P/L headline plus the value-over-time graph, aggregated
// from the list payload's cost basis + one household-scoped time-series fetch.
function InvestmentHeadline<T>({
  items,
  config,
}: {
  items: T[];
  config: InvestmentConfig<T>;
}) {
  const { t } = useTranslation(config.i18nNamespaces);
  const timeSeries = useInvestmentTimeSeries();
  const positions = useMemo<Position[]>(
    () =>
      items.map((item) => {
        const core = config.entity(item);
        const ts = timeSeries.byId.get(core.id);
        const snap = config.getSnapshot(item);
        return {
          id: core.id,
          currency: core.native_currency,
          status: core.status,
          terminated_at: core.terminated_at,
          latestValue: snap ? Number(snap.amount) : null,
          cost: config.costBasis(item),
          snapshots: ts?.snapshots ?? [],
          costSeries: ts?.costSeries ?? [],
        };
      }),
    [items, timeSeries.byId, config],
  );
  const aggregates = useMemo(
    () => aggregateListPositions(positions),
    [positions],
  );
  return (
    <>
      <InvestmentListHeadline
        aggregates={aggregates.byCurrency}
        count={aggregates.count}
        noun={t(config.keys.noun)}
        nounPlural={t(config.keys.nounPlural)}
        testId={config.headlineTestId}
      />
      <ListTimeGraph timeSeriesByCurrency={aggregates.timeSeriesByCurrency} />
    </>
  );
}

export function investmentDescriptor<T>(
  config: InvestmentConfig<T>,
): PositionListDescriptor<T, void> {
  const activityColumn: PositionExtraColumn<T, void> = {
    id: "activity",
    labelKey: "common:activity.header",
    slot: "trailing",
    render: (item) => <ActivityContent {...config.activity(item)} />,
  };
  return {
    entityKey: config.entityKey,
    testIdPrefix: config.testIdPrefix,
    group: "investments",
    i18nNamespaces: config.i18nNamespaces,
    defaultSortKey: "name",
    keys: config.keys,
    useList: config.useList,
    useDelete: config.useDelete,
    useImport: config.useImport,
    useRowFilter: () => {
      const [value, setValue] = useState<RiskProfileFilterValue>("all");
      return {
        control: <RiskProfileFilter value={value} onChange={setValue} />,
        predicate: (item: T) =>
          value === "all" || config.entity(item).risk_profile === value,
      };
    },
    getId: (item) => config.entity(item).id,
    getName: (item) => config.entity(item).display_name,
    getStatus: (item) => config.entity(item).status,
    getSnapshot: config.getSnapshot,
    getSecondary: (item) => config.entity(item).description ?? "",
    deleteDescription: config.deleteDescription,
    extraColumns: [...(config.mainColumns ?? []), activityColumn],
    renderTitleAccessory: (item) => (
      <RiskProfileBadge profile={config.entity(item).risk_profile} compact />
    ),
    renderHeadline: (items) => (
      <InvestmentHeadline items={items} config={config} />
    ),
    renderCreateDialog: config.renderCreateDialog,
    renderEditDialog: config.renderEditDialog,
  };
}
