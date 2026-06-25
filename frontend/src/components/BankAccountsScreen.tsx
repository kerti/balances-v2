// i18n template (issue #7). The per-group extraction pattern lives here and
// is reused by the other position groups (#8 properties/vehicles, #9
// liabilities/receivables, #10 investments, #11 income):
//
// • Group-specific copy → namespace per group (`assets`, `liabilities`,
//   `investments`, `income`). Bank-account text lives at
//   `assets.bankAccount.*`.
// • Shared field labels (reused across ≥2 groups) → `common.fields.*`.
// • Shared dialogs (snapshot / terminate / import) → `common.snapshot.*`,
//   `common.terminate.*`, `common.import.*` — Create/Edit/RowSnapshot,
//   TerminatePositionDialog and ImportSnapshotsDialog all read from these.
// • Lifecycle status labels → `common.lifecycle.<group>.<status>` (resolved
//   via lib/lifecycle.ts → i18n.t with a defaultValue fallback).
// • Ownership labels → `common.ownership.*` (resolved via lib/ownership.ts).
// • Error / toast copy → `errors.*`.
//
// Plural-sensitive lines use i18next's `_one` / `_other` suffix; counts are
// passed as the `count` interpolation key. Single-noun screens pass their
// own noun/nounPlural pair into the shared list keys.
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
import {
  useBankAccounts,
  useImportCreateBankAccount,
} from "@/hooks/useBankAccounts";
import { useHouseholdMembers } from "@/hooks/useHouseholdMembers";
import { useSession } from "@/hooks/useSession";
import { useTableSort, type ColumnSort } from "@/hooks/useTableSort";
import { CreateBankAccountDialog } from "@/components/CreateBankAccountDialog";
import { ImportPositionDialog } from "@/components/ImportPositionDialog";
import { BankAccountListRow } from "@/components/BankAccountListRow";
import { ownershipLabel } from "@/lib/ownership";
import { isActiveStatus, statusLabel } from "@/lib/lifecycle";
import { activeCurrencyTotals } from "@/lib/totals";
import { byNumberNullsLast, byText } from "@/lib/sort";
import type { BankAccountListItem } from "@/api/types";

type Props = {
  onSelect: (id: string) => void;
};

type SortKey = "name" | "ownership" | "status" | "balance";

type Row = {
  item: BankAccountListItem;
  ownerLabel: string;
  name: string;
  status: string;
  statusText: string;
  amount: number | null;
};

const tiebreakByName = (a: Row, b: Row) => a.name.localeCompare(b.name);

export function BankAccountsScreen({ onSelect }: Props) {
  const { t } = useTranslation(["assets", "common", "errors"]);
  const { data, isPending, error } = useBankAccounts();
  const importMutation = useImportCreateBankAccount();
  const { data: members } = useHouseholdMembers();
  const { data: currentUser } = useSession();
  const [showInactive, setShowInactive] = useState(false);

  const noun = t("assets:bankAccount.noun");
  const nounPlural = t("assets:bankAccount.nounPlural");

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

  const terminatedCount = rows.filter((r) => !isActiveStatus(r.status)).length;
  const visibleRows = showInactive
    ? sorted
    : sorted.filter((r) => isActiveStatus(r.status));

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">
            {t("assets:bankAccount.listTitle")}
          </h1>
          <p className="text-sm text-muted-foreground">
            {t("assets:bankAccount.listSubtitle")}
          </p>
        </div>
        <div className="flex items-center gap-2">
          <ImportPositionDialog noun={noun} mutation={importMutation} />
          <CreateBankAccountDialog />
        </div>
      </div>

      <ListHeadline
        totals={totals}
        count={count}
        label={t("assets:bankAccount.totalBalance")}
        noun={noun}
        nounPlural={nounPlural}
        testId="bank-accounts-total"
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
            <CardTitle>{t("assets:bankAccount.emptyTitle")}</CardTitle>
            <CardDescription>
              {t("assets:bankAccount.emptyBody")}
            </CardDescription>
          </CardHeader>
          <CardContent>
            <CreateBankAccountDialog />
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
                        label={t("assets:bankAccount.sortLatestBalance")}
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
                      <BankAccountListRow
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
