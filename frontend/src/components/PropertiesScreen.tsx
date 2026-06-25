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
import { useProperties, useImportCreateProperty } from "@/hooks/useProperties";
import { useHouseholdMembers } from "@/hooks/useHouseholdMembers";
import { useSession } from "@/hooks/useSession";
import { useTableSort, type ColumnSort } from "@/hooks/useTableSort";
import { CreatePropertyDialog } from "@/components/CreatePropertyDialog";
import { ImportPositionDialog } from "@/components/ImportPositionDialog";
import { PropertyListRow } from "@/components/PropertyListRow";
import { ownershipLabel } from "@/lib/ownership";
import { isActiveStatus, statusLabel } from "@/lib/lifecycle";
import { activeCurrencyTotals } from "@/lib/totals";
import { byNumberNullsLast, byText } from "@/lib/sort";
import type { PropertyListItem } from "@/api/types";

type Props = {
  onSelect: (id: string) => void;
};

type SortKey = "name" | "ownership" | "status" | "value";

type Row = {
  item: PropertyListItem;
  ownerLabel: string;
  name: string;
  status: string;
  statusText: string;
  amount: number | null;
};

const tiebreakByName = (a: Row, b: Row) => a.name.localeCompare(b.name);

export function PropertiesScreen({ onSelect }: Props) {
  const { t } = useTranslation(["assets", "common", "errors"]);
  const { data, isPending, error } = useProperties();
  const importMutation = useImportCreateProperty();
  const { data: members } = useHouseholdMembers();
  const { data: currentUser } = useSession();
  const [showInactive, setShowInactive] = useState(false);

  const noun = t("assets:property.noun");
  const nounPlural = t("assets:property.nounPlural");

  const rows = useMemo<Row[]>(
    () =>
      (data ?? []).map((item) => ({
        item,
        ownerLabel: ownershipLabel(
          item.asset.ownership_type,
          item.asset.sole_owner_user_id,
          members,
          currentUser,
        ),
        name: item.asset.display_name,
        status: item.asset.status,
        statusText: statusLabel("assets", item.asset.status),
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
      value: { dir: "desc", cmp: byNumberNullsLast((r) => r.amount) },
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

  const terminatedCount = rows.filter((r) => !isActiveStatus(r.status)).length;
  const visibleRows = showInactive
    ? sorted
    : sorted.filter((r) => isActiveStatus(r.status));

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">
            {t("assets:property.listTitle")}
          </h1>
          <p className="text-sm text-muted-foreground">
            {t("assets:property.listSubtitle")}
          </p>
        </div>
        <div className="flex items-center gap-2">
          <ImportPositionDialog noun={noun} mutation={importMutation} />
          <CreatePropertyDialog />
        </div>
      </div>

      <ListHeadline
        totals={totals}
        count={count}
        label={t("assets:property.totalValue")}
        noun={noun}
        nounPlural={nounPlural}
        testId="properties-total"
      />

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
            <CardTitle>{t("assets:property.emptyTitle")}</CardTitle>
            <CardDescription>{t("assets:property.emptyBody")}</CardDescription>
          </CardHeader>
          <CardContent>
            <CreatePropertyDialog />
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
                        label={t("assets:property.sortLatestValuation")}
                        testId="sort-value"
                        align="right"
                        active={sortKey === "value"}
                        dir={sortDir}
                        onSort={() => toggle("value")}
                      />
                      <TableHead className="w-12"></TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {visibleRows.map((r) => (
                      <PropertyListRow
                        key={r.item.asset.id}
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
