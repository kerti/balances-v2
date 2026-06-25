import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { SortableHeader } from "@/components/SortableHeader";
import { ListHeadline } from "@/components/ListHeadline";
import { ShowInactiveToggle } from "@/components/ShowInactiveToggle";
import { SnapshotChart } from "@/components/SnapshotChart";
import { aggregateListPositions } from "@/lib/listAggregates";
import {
  useReceivables,
  useImportCreateReceivable,
} from "@/hooks/useReceivables";
import { useReceivableTimeSeries } from "@/hooks/useReceivableTimeSeries";
import { useHouseholdMembers } from "@/hooks/useHouseholdMembers";
import { useSession } from "@/hooks/useSession";
import { useTableSort, type ColumnSort } from "@/hooks/useTableSort";
import { CreateReceivableDialog } from "@/components/CreateReceivableDialog";
import { ImportPositionDialog } from "@/components/ImportPositionDialog";
import { ReceivableListRow } from "@/components/ReceivableListRow";
import { ownershipLabel } from "@/lib/ownership";
import { isActiveStatus, statusLabel } from "@/lib/lifecycle";
import { activeCurrencyTotals } from "@/lib/totals";
import { byNumberNullsLast, byText } from "@/lib/sort";
import type { ReceivableListItem } from "@/api/types";

type Props = {
  onSelect: (id: string) => void;
};

type SortKey = "name" | "ownership" | "status" | "balance";

type Row = {
  item: ReceivableListItem;
  ownerLabel: string;
  name: string;
  status: string;
  statusText: string;
  amount: number | null;
};

const tiebreakByName = (a: Row, b: Row) => a.name.localeCompare(b.name);

export function ReceivablesScreen({ onSelect }: Props) {
  const { t } = useTranslation(["receivables", "common", "errors"]);
  const { data, isPending, error } = useReceivables();
  const timeSeries = useReceivableTimeSeries();
  const importMutation = useImportCreateReceivable();
  const { data: members } = useHouseholdMembers();
  const { data: currentUser } = useSession();
  const [showInactive, setShowInactive] = useState(false);

  const noun = t("receivables:noun");
  const nounPlural = t("receivables:nounPlural");

  const rows = useMemo<Row[]>(
    () =>
      (data ?? []).map((item) => ({
        item,
        ownerLabel: ownershipLabel(
          item.receivable.ownership_type,
          item.receivable.sole_owner_user_id,
          members,
          currentUser,
        ),
        name: item.receivable.display_name,
        status: item.receivable.status,
        statusText: statusLabel("receivables", item.receivable.status),
        amount: item.latest_snapshot
          ? Number(item.latest_snapshot.amount)
          : null,
      })),
    [data, members, currentUser],
  );

  const columns = useMemo<Record<SortKey, ColumnSort<Row>>>(
    () => ({
      name: { dir: "asc", cmp: byText((r) => r.name) },
      ownership: { dir: "asc", cmp: byText((r) => r.ownerLabel) },
      status: { dir: "asc", cmp: byText((r) => r.statusText) },
      balance: { dir: "desc", cmp: byNumberNullsLast((r) => r.amount) },
    }),
    [],
  );

  const { sorted, sortKey, sortDir, toggle } = useTableSort(rows, columns, {
    defaultKey: "name",
    tiebreak: tiebreakByName,
  });

  const { totals, count } = useMemo(
    () =>
      activeCurrencyTotals(
        rows.map((r) => ({
          status: r.status,
          snapshot: r.item.latest_snapshot,
        })),
      ),
    [rows],
  );

  // Total-outstanding over time, per native currency (epic #204). Receivables
  // are a flat group — one line per currency, no subtype stack/pie. Value-only
  // (no cost basis), so aggregateListPositions runs with cost omitted.
  const timeSeriesByCurrency = useMemo(() => {
    const positions = (data ?? []).map((item) => ({
      id: item.receivable.id,
      currency: item.receivable.native_currency,
      status: item.receivable.status,
      terminated_at: item.receivable.terminated_at,
      latestValue: item.latest_snapshot
        ? Number(item.latest_snapshot.amount)
        : null,
      snapshots: timeSeries.byId.get(item.receivable.id)?.snapshots ?? [],
    }));
    return aggregateListPositions(positions).timeSeriesByCurrency;
  }, [data, timeSeries.byId]);

  const chartCurrencies = [...timeSeriesByCurrency.keys()].sort();

  const terminatedCount = rows.filter((r) => !isActiveStatus(r.status)).length;
  const visibleRows = showInactive
    ? sorted
    : sorted.filter((r) => isActiveStatus(r.status));

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">
            {t("receivables:listTitle")}
          </h1>
          <p className="text-sm text-muted-foreground">
            {t("receivables:listSubtitle")}
          </p>
        </div>
        <div className="flex items-center gap-2">
          <ImportPositionDialog noun={noun} mutation={importMutation} />
          <CreateReceivableDialog />
        </div>
      </div>

      <ListHeadline
        totals={totals}
        count={count}
        label={t("receivables:totalOutstanding")}
        noun={noun}
        nounPlural={nounPlural}
        testId="receivables-total"
      />

      {chartCurrencies.map((currency) => {
        const series = timeSeriesByCurrency.get(currency) ?? [];
        if (series.length < 2) return null;
        return (
          <Card
            key={currency}
            data-testid={`receivables-value-chart-${currency}`}
          >
            <CardHeader>
              <CardTitle>{t("receivables:home.valueChartTitle")}</CardTitle>
              <CardDescription>
                {t("receivables:home.valueChartDescription", { currency })}
              </CardDescription>
            </CardHeader>
            <CardContent>
              <SnapshotChart
                snapshots={series.map((p) => ({
                  year_month: p.year_month,
                  amount: String(p.value),
                }))}
                currency={currency}
              />
            </CardContent>
          </Card>
        );
      })}

      {isPending && (
        <p className="text-sm text-muted-foreground">{t("common:loading")}</p>
      )}

      {error && (
        <p className="text-sm text-destructive">
          {t("errors:failedToLoad", { message: (error as Error).message })}
        </p>
      )}

      {data && data.length === 0 && (
        <Card>
          <CardHeader>
            <CardTitle>{t("receivables:emptyTitle")}</CardTitle>
            <CardDescription>{t("receivables:emptyBody")}</CardDescription>
          </CardHeader>
          <CardContent>
            <CreateReceivableDialog />
          </CardContent>
        </Card>
      )}

      {data && data.length > 0 && (
        <div className="space-y-3">
          {terminatedCount > 0 && (
            <ShowInactiveToggle
              count={terminatedCount}
              nounPlural={nounPlural}
              checked={showInactive}
              onChange={setShowInactive}
            />
          )}

          {visibleRows.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              {t("common:list.noActive", {
                count: terminatedCount,
                noun,
                nounPlural,
              })}
            </p>
          ) : (
            <Card>
              <CardContent className="p-0">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <SortableHeader
                        label={t("common:tableHeaders.name")}
                        testId="sort-name"
                        active={sortKey === "name"}
                        dir={sortDir}
                        onSort={() => toggle("name")}
                      />
                      <SortableHeader
                        label={t("common:tableHeaders.ownership")}
                        testId="sort-ownership"
                        active={sortKey === "ownership"}
                        dir={sortDir}
                        onSort={() => toggle("ownership")}
                      />
                      <SortableHeader
                        label={t("common:tableHeaders.status")}
                        testId="sort-status"
                        active={sortKey === "status"}
                        dir={sortDir}
                        onSort={() => toggle("status")}
                      />
                      <SortableHeader
                        label={t("receivables:sortLatestBalance")}
                        testId="sort-balance"
                        align="right"
                        active={sortKey === "balance"}
                        dir={sortDir}
                        onSort={() => toggle("balance")}
                      />
                      <TableHead className="w-12"></TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {visibleRows.map((r) => (
                      <ReceivableListRow
                        key={r.item.receivable.id}
                        item={r.item}
                        ownerLabel={r.ownerLabel}
                        onSelect={onSelect}
                      />
                    ))}
                  </TableBody>
                </Table>
              </CardContent>
            </Card>
          )}
        </div>
      )}
    </div>
  );
}
