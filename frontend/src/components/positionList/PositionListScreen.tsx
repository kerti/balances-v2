import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { ShowInactiveToggle } from "@/components/ShowInactiveToggle";
import { StatusBadge } from "@/components/StatusBadge";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import { ImportPositionDialog } from "@/components/ImportPositionDialog";
import { PositionListTable } from "@/components/positionList/PositionListTable";
import { PositionListCards } from "@/components/positionList/PositionListCards";
import { useIsMobile } from "@/hooks/use-mobile";
import { useTableSort, type ColumnSort } from "@/hooks/useTableSort";
import { formatCurrency, formatYearMonth } from "@/lib/format";
import { isActiveStatus, statusLabel } from "@/lib/lifecycle";
import { byNumberNullsLast, byText } from "@/lib/sort";
import { cn } from "@/lib/utils";
import type {
  ColumnView,
  PositionListDescriptor,
  RowView,
} from "@/components/positionList/types";

type Props<T, Ctx> = {
  descriptor: PositionListDescriptor<T, Ctx>;
  onSelect: (id: string) => void;
};

// The generic Position list screen (ADR-0043). It owns every shared-surface
// concern — the header, the headline slot, loading/empty/error, the
// show-inactive toggle, the sort wiring, the ⋮ → edit/delete dialogs — and the
// four shared columns as hard JSX below. A descriptor supplies only wiring and
// slots; group-specific fields arrive as extra columns the core places but
// never inspects. `useIsMobile` picks the renderer; both consume one
// presentation-neutral column list.
export function PositionListScreen<T, Ctx>({
  descriptor,
  onSelect,
}: Props<T, Ctx>) {
  const { t } = useTranslation(descriptor.i18nNamespaces);
  const { data, isPending, error } = descriptor.useList();
  const deleteMutation = descriptor.useDelete();
  const importMutation = descriptor.useImport?.();
  const ctx = (descriptor.useExtraContext?.() ?? undefined) as Ctx;
  const rowFilter = descriptor.useRowFilter?.();
  const isMobile = useIsMobile();

  const [showInactive, setShowInactive] = useState(false);
  const [editItem, setEditItem] = useState<T | null>(null);
  const [deleteItem, setDeleteItem] = useState<T | null>(null);

  const { keys } = descriptor;
  const noun = t(keys.noun);
  const nounPlural = t(keys.nounPlural);

  const rows = useMemo<RowView<T>[]>(
    () =>
      (data ?? []).map((item) => {
        const status = descriptor.getStatus(item);
        const snapshot = descriptor.getSnapshot(item);
        return {
          item,
          id: descriptor.getId(item),
          name: descriptor.getName(item),
          secondary: descriptor.getSecondary(item, t),
          status,
          statusText: statusLabel(descriptor.group, status),
          amount: snapshot ? Number(snapshot.amount) : null,
          snapshot,
          terminated: !isActiveStatus(status),
        };
      }),
    [data, descriptor, t],
  );

  // The four shared columns are hard JSX here — never config — plus the
  // descriptor's extra columns slotted in around them. "main" extras go between
  // name and status; "trailing" extras after value.
  const columns = useMemo<ColumnView<T>[]>(() => {
    const nameCol: ColumnView<T> = {
      id: "name",
      label: t("common:tableHeaders.name"),
      align: "left",
      sortKey: "name",
      isTitle: true,
      mobileVisible: true,
      cell: (row) => (
        <>
          <div className={cn("font-medium", row.terminated && "font-normal")}>
            {row.name}
          </div>
          <div className="text-xs text-muted-foreground">{row.secondary}</div>
        </>
      ),
    };
    const statusCol: ColumnView<T> = {
      id: "status",
      label: t("common:tableHeaders.status"),
      align: "left",
      sortKey: "status",
      mobileVisible: true,
      cell: (row) => (
        <StatusBadge group={descriptor.group} status={row.status} />
      ),
    };
    const valueCol: ColumnView<T> = {
      id: "value",
      label: t(keys.valueLabel),
      align: "right",
      sortKey: "value",
      mobileVisible: true,
      cell: (row) =>
        row.snapshot ? (
          <>
            <div>
              {formatCurrency(row.snapshot.amount, row.snapshot.currency)}
            </div>
            <div className="text-xs text-muted-foreground">
              {formatYearMonth(row.snapshot.year_month)}
            </div>
          </>
        ) : (
          <span className="text-muted-foreground">{"—"}</span>
        ),
    };

    const extra = (slot: "main" | "trailing") =>
      descriptor.extraColumns
        .filter((col) => (col.slot ?? "main") === slot)
        .map<ColumnView<T>>((col) => ({
          id: col.id,
          label: t(col.labelKey),
          align: col.align ?? "left",
          sortKey: col.sort?.key,
          mobileVisible: col.mobile === "secondary",
          cell: (row) => col.render(row.item, ctx),
        }));

    return [
      nameCol,
      ...extra("main"),
      statusCol,
      valueCol,
      ...extra("trailing"),
    ];
  }, [descriptor, keys.valueLabel, t, ctx]);

  const sortColumns = useMemo<Record<string, ColumnSort<RowView<T>>>>(() => {
    const map: Record<string, ColumnSort<RowView<T>>> = {
      name: { dir: "asc", cmp: byText((r) => r.name) },
      status: { dir: "asc", cmp: byText((r) => r.statusText) },
      value: { dir: "desc", cmp: byNumberNullsLast((r) => r.amount) },
    };
    for (const col of descriptor.extraColumns) {
      if (!col.sort) continue;
      const { key, type, value } = col.sort;
      map[key] =
        type === "number"
          ? {
              dir: "asc",
              cmp: byNumberNullsLast(
                (r) => value(r.item, ctx) as number | null,
              ),
            }
          : {
              dir: "asc",
              cmp: byText((r) => String(value(r.item, ctx) ?? "")),
            };
    }
    return map;
  }, [descriptor, ctx]);

  const tiebreak = useMemo(
    () => (a: RowView<T>, b: RowView<T>) => a.name.localeCompare(b.name),
    [],
  );

  const { sorted, sortKey, sortDir, toggle } = useTableSort(rows, sortColumns, {
    defaultKey: descriptor.defaultSortKey,
    tiebreak,
  });

  const terminatedCount = rows.filter((r) => r.terminated).length;
  let visibleRows = showInactive ? sorted : sorted.filter((r) => !r.terminated);
  if (rowFilter)
    visibleRows = visibleRows.filter((r) => rowFilter.predicate(r.item));

  function handleDeleteConfirm() {
    if (!deleteItem) return;
    deleteMutation.mutate(descriptor.getId(deleteItem), {
      onSuccess: () => setDeleteItem(null),
    });
  }

  // Both renderers share one signature; the cast picks one without tripping
  // generic-through-union inference on the ternary.
  const Renderer = (
    isMobile ? PositionListCards : PositionListTable
  ) as typeof PositionListTable;

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">
            {t(keys.listTitle)}
          </h1>
          <p className="text-sm text-muted-foreground">
            {t(keys.listSubtitle)}
          </p>
        </div>
        <div className="flex items-center gap-2">
          {importMutation && (
            <ImportPositionDialog noun={noun} mutation={importMutation} />
          )}
          {descriptor.renderCreateDialog()}
        </div>
      </div>

      {descriptor.renderHeadline(data ?? [])}

      {isPending && (
        <p className="text-sm text-muted-foreground">{t("common:loading")}</p>
      )}

      {error != null && (
        <p className="text-sm text-destructive">
          {t("errors:failedToLoad", { message: (error as Error).message })}
        </p>
      )}

      {data && data.length === 0 && (
        <Card>
          <CardHeader>
            <CardTitle>{t(keys.emptyTitle)}</CardTitle>
            <CardDescription>{t(keys.emptyBody)}</CardDescription>
          </CardHeader>
          <CardContent>{descriptor.renderCreateDialog()}</CardContent>
        </Card>
      )}

      {data && data.length > 0 && (
        <div className="space-y-3">
          {(terminatedCount > 0 || rowFilter) && (
            <div className="flex items-center justify-between gap-4">
              {rowFilter?.control ?? <span />}
              {terminatedCount > 0 && (
                <ShowInactiveToggle
                  count={terminatedCount}
                  nounPlural={nounPlural}
                  checked={showInactive}
                  onChange={setShowInactive}
                />
              )}
            </div>
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
            <Renderer
              rows={visibleRows}
              columns={columns}
              sortKey={sortKey}
              sortDir={sortDir}
              onToggleSort={toggle}
              onSelect={onSelect}
              onEdit={setEditItem}
              onDelete={setDeleteItem}
              actionsLabel={t(keys.rowActions)}
              testIdPrefix={descriptor.testIdPrefix}
            />
          )}
        </div>
      )}

      {editItem !== null &&
        descriptor.renderEditDialog(editItem, {
          open: true,
          onOpenChange: (open) => {
            if (!open) setEditItem(null);
          },
        })}

      <ConfirmDialog
        open={deleteItem !== null}
        onOpenChange={(open) => {
          if (!open) setDeleteItem(null);
        }}
        title={t(keys.deleteTitle)}
        description={
          deleteItem ? descriptor.deleteDescription(deleteItem, t) : undefined
        }
        confirmLabel={t("common:delete")}
        destructive
        pending={deleteMutation.isPending}
        onConfirm={handleDeleteConfirm}
      />
    </div>
  );
}
