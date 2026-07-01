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
import { Button } from "@/components/ui/button";
import { useIncome } from "@/hooks/useIncome";
import { useHouseholdMembers } from "@/hooks/useHouseholdMembers";
import { useSession } from "@/hooks/useSession";
import { CreateIncomeDialog } from "@/components/CreateIncomeDialog";
import { IncomeRow } from "@/components/IncomeRow";
import { MonthPickerPopover } from "@/components/MonthPickerPopover";
import { PaginationControls } from "@/components/PaginationControls";
import { ownershipLabel } from "@/lib/ownership";
import { formatCurrency } from "@/lib/format";
import type { Income, IncomeCategory, Regularity } from "@/api/types";

const PAGE_SIZE = 12;

type RegularityFilter = "all" | Regularity;

const FILTER_VALUES: RegularityFilter[] = ["all", "routine", "incidental"];

type HeadlineCurrency = {
  currency: string;
  total: number;
  routine: number;
  incidental: number;
  byUser: Array<{ label: string; amount: number }>;
  byCategory: Array<{ category: IncomeCategory; amount: number }>;
};

function incomeYearMonth(r: Income): string {
  return r.date.slice(0, 7);
}

export function IncomeScreen() {
  const { t } = useTranslation(["income", "common", "errors"]);
  const { data, isPending, error } = useIncome();
  const { data: members } = useHouseholdMembers();
  const { data: currentUser } = useSession();
  const [page, setPage] = useState(1);
  const [regularityFilter, setRegularityFilter] =
    useState<RegularityFilter>("all");
  // undefined = not yet set by user → auto-picks most recent month once data loads
  // string = specific "YYYY-MM"
  const [selectedMonth, setSelectedMonth] = useState<string | undefined>(
    undefined,
  );

  const availableMonths = useMemo(() => {
    if (!data) return [];
    return [...new Set(data.map(incomeYearMonth))].sort((a, b) =>
      b.localeCompare(a),
    );
  }, [data]);

  const effectiveMonth: string | undefined =
    selectedMonth ?? availableMonths[0];

  const monthFiltered = useMemo(() => {
    if (!data || !effectiveMonth) return [];
    return data.filter((r) => incomeYearMonth(r) === effectiveMonth);
  }, [data, effectiveMonth]);

  const headlineStats = useMemo((): HeadlineCurrency[] => {
    if (!monthFiltered.length) return [];
    const byCurrency = new Map<
      string,
      {
        total: number;
        routine: number;
        incidental: number;
        byUser: Map<string, number>;
        byCategory: Map<IncomeCategory, number>;
      }
    >();
    for (const r of monthFiltered) {
      const amount = Number(r.amount);
      if (!Number.isFinite(amount)) continue;
      let cur = byCurrency.get(r.currency);
      if (!cur) {
        cur = {
          total: 0,
          routine: 0,
          incidental: 0,
          byUser: new Map(),
          byCategory: new Map(),
        };
        byCurrency.set(r.currency, cur);
      }
      cur.total += amount;
      if (r.regularity === "routine") cur.routine += amount;
      else cur.incidental += amount;
      const userLabel = ownershipLabel(
        r.ownership_type,
        r.sole_owner_user_id,
        members,
        currentUser,
      );
      cur.byUser.set(userLabel, (cur.byUser.get(userLabel) ?? 0) + amount);
      cur.byCategory.set(
        r.category,
        (cur.byCategory.get(r.category) ?? 0) + amount,
      );
    }
    return Array.from(byCurrency.entries())
      .map(([currency, d]) => ({
        currency,
        total: d.total,
        routine: d.routine,
        incidental: d.incidental,
        byUser: Array.from(d.byUser.entries())
          .map(([label, amount]) => ({ label, amount }))
          .sort((a, b) => b.amount - a.amount),
        byCategory: Array.from(d.byCategory.entries())
          .map(([category, amount]) => ({ category, amount }))
          .sort((a, b) => b.amount - a.amount),
      }))
      .sort((a, b) => a.currency.localeCompare(b.currency));
  }, [monthFiltered, members, currentUser]);

  const filtered = useMemo(() => {
    const base =
      regularityFilter === "all"
        ? monthFiltered
        : monthFiltered.filter((r) => r.regularity === regularityFilter);
    return [...base].sort((a, b) => {
      const uA = ownershipLabel(
        a.ownership_type,
        a.sole_owner_user_id,
        members,
        currentUser,
      );
      const uB = ownershipLabel(
        b.ownership_type,
        b.sole_owner_user_id,
        members,
        currentUser,
      );
      const cmp = uA.localeCompare(uB);
      return cmp !== 0 ? cmp : a.date.localeCompare(b.date);
    });
  }, [monthFiltered, regularityFilter, members, currentUser]);

  const totalPages = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE));
  const effectivePage = Math.min(page, totalPages);
  const pageRows = filtered.slice(
    (effectivePage - 1) * PAGE_SIZE,
    effectivePage * PAGE_SIZE,
  );

  const emptyKey =
    regularityFilter === "routine"
      ? "income:filter.emptyRoutine"
      : regularityFilter === "incidental"
        ? "income:filter.emptyIncidental"
        : "income:filter.emptyAll";

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">
            {t("income:listTitle")}
          </h1>
          <p className="text-sm text-muted-foreground">
            {t("income:listSubtitle")}
          </p>
        </div>
        <CreateIncomeDialog />
      </div>

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
            <CardTitle>{t("income:emptyTitle")}</CardTitle>
            <CardDescription>{t("income:emptyBody")}</CardDescription>
          </CardHeader>
          <CardContent>
            <CreateIncomeDialog />
          </CardContent>
        </Card>
      )}

      {data && data.length > 0 && (
        <div className="space-y-3">
          <div className="flex flex-wrap items-center gap-2">
            {effectiveMonth && (
              <MonthPickerPopover
                months={availableMonths}
                selected={effectiveMonth}
                onSelect={(ym) => {
                  setSelectedMonth(ym);
                  setPage(1);
                }}
              />
            )}

            <div
              className="flex gap-2"
              role="group"
              aria-label={t("income:filter.ariaLabel")}
            >
              {FILTER_VALUES.map((value) => (
                <Button
                  key={value}
                  size="sm"
                  variant={regularityFilter === value ? "default" : "outline"}
                  onClick={() => {
                    setRegularityFilter(value);
                    setPage(1);
                  }}
                  data-testid={`regularity-filter-${value}`}
                >
                  {t(`income:filter.${value}`)}
                </Button>
              ))}
            </div>
          </div>

          {headlineStats.length > 0 && (
            <div className="flex flex-wrap gap-4">
              {headlineStats.map((h) => (
                <Card key={h.currency} className="flex-1 min-w-60">
                  <CardContent className="pt-4 space-y-2">
                    <div>
                      <div className="text-xs uppercase tracking-wide text-muted-foreground">
                        {t("income:headline.total")}
                      </div>
                      <div className="text-2xl font-semibold tabular-nums">
                        {formatCurrency(String(h.total), h.currency)}
                      </div>
                    </div>
                    <div className="flex flex-wrap gap-x-4 text-sm">
                      <span>
                        <span className="text-muted-foreground">
                          {t("income:headline.routine")}
                        </span>{" "}
                        <span className="tabular-nums">
                          {formatCurrency(String(h.routine), h.currency)}
                        </span>
                      </span>
                      <span>
                        <span className="text-muted-foreground">
                          {t("income:headline.incidental")}
                        </span>{" "}
                        <span className="tabular-nums">
                          {formatCurrency(String(h.incidental), h.currency)}
                        </span>
                      </span>
                    </div>
                    {h.byUser.length > 1 && (
                      <div className="text-sm">
                        <span className="text-muted-foreground">
                          {t("income:headline.byPerson")}
                          {": "}
                        </span>
                        {h.byUser.map((u, i) => (
                          <span key={u.label}>
                            {i > 0 && " · "}
                            {u.label}{" "}
                            <span className="tabular-nums">
                              {formatCurrency(String(u.amount), h.currency)}
                            </span>
                          </span>
                        ))}
                      </div>
                    )}
                    <div className="text-sm">
                      <span className="text-muted-foreground">
                        {t("income:headline.byCategory")}
                        {": "}
                      </span>
                      {h.byCategory.map((c, i) => (
                        <span key={c.category}>
                          {i > 0 && " · "}
                          {t(`income:categories.${c.category}`)}{" "}
                          <span className="tabular-nums">
                            {formatCurrency(String(c.amount), h.currency)}
                          </span>
                        </span>
                      ))}
                    </div>
                  </CardContent>
                </Card>
              ))}
            </div>
          )}

          {filtered.length === 0 ? (
            <p className="text-sm text-muted-foreground">{t(emptyKey)}</p>
          ) : (
            <Card>
              <CardContent className="p-0">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t("income:tableHeaders.date")}</TableHead>
                      <TableHead>{t("income:tableHeaders.category")}</TableHead>
                      <TableHead>{t("income:tableHeaders.amount")}</TableHead>
                      <TableHead>
                        {t("income:tableHeaders.description")}
                      </TableHead>
                      <TableHead>
                        {t("income:tableHeaders.ownership")}
                      </TableHead>
                      <TableHead className="w-12"></TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {pageRows.map((row) => (
                      <IncomeRow key={row.id} income={row} />
                    ))}
                  </TableBody>
                </Table>
                {totalPages > 1 && (
                  <div className="border-t px-6 py-3">
                    <PaginationControls
                      page={effectivePage}
                      totalPages={totalPages}
                      onPageChange={setPage}
                    />
                  </div>
                )}
              </CardContent>
            </Card>
          )}
        </div>
      )}
    </div>
  );
}
