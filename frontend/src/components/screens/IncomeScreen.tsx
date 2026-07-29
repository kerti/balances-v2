import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { useIncome } from "@/hooks/useIncome";
import { useHouseholdMembers } from "@/hooks/useHouseholdMembers";
import { useSession } from "@/hooks/useSession";
import { CreateIncomeDialog } from "@/components/dialogs/CreateIncomeDialog";
import { IncomeList } from "@/components/income/IncomeList";
import { MonthPickerPopover } from "@/components/common/MonthPickerPopover";
import { useIsMobile } from "@/hooks/use-mobile";
import { ownershipLabel } from "@/lib/ownership";
import { formatCurrency } from "@/lib/format";
import { headlineSurface } from "@/lib/headline";
import { cn } from "@/lib/utils";
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
  const isMobile = useIsMobile();
  const [page, setPage] = useState(1);
  const [regularityFilter, setRegularityFilter] = useState<RegularityFilter>("all");
  // undefined = not yet set by user → auto-picks most recent month once data loads
  // string = specific "YYYY-MM"
  const [selectedMonth, setSelectedMonth] = useState<string | undefined>(undefined);

  const availableMonths = useMemo(() => {
    if (!data) return [];
    return [...new Set(data.map(incomeYearMonth))].sort((a, b) => b.localeCompare(a));
  }, [data]);

  const effectiveMonth: string | undefined = selectedMonth ?? availableMonths[0];

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
      cur.byCategory.set(r.category, (cur.byCategory.get(r.category) ?? 0) + amount);
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
      const uA = ownershipLabel(a.ownership_type, a.sole_owner_user_id, members, currentUser);
      const uB = ownershipLabel(b.ownership_type, b.sole_owner_user_id, members, currentUser);
      const cmp = uA.localeCompare(uB);
      return cmp !== 0 ? cmp : a.date.localeCompare(b.date);
    });
  }, [monthFiltered, regularityFilter, members, currentUser]);

  const totalPages = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE));
  const effectivePage = Math.min(page, totalPages);
  const pageRows = filtered.slice((effectivePage - 1) * PAGE_SIZE, effectivePage * PAGE_SIZE);

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
          <h1 className="text-2xl font-semibold tracking-tight">{t("income:listTitle")}</h1>
          <p className="text-sm text-muted-foreground">{t("income:listSubtitle")}</p>
        </div>
        {/* Desktop: the primary action lives top-right. On mobile it moves onto
            the filter toolbar as a compact "New" (see below) to reclaim the
            vertical space under the page copy. */}
        {!isMobile && <CreateIncomeDialog />}
      </div>

      {isPending && <p className="text-sm text-muted-foreground">{t("common:loading")}</p>}

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

            <div className="flex gap-2" role="group" aria-label={t("income:filter.ariaLabel")}>
              {FILTER_VALUES.map((value) => (
                <Button
                  key={value}
                  size="sm"
                  className="min-h-11 md:min-h-0"
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

            {/* Mobile: the create action rides the filter toolbar, pushed to
                the right edge; the `+` icon + primary fill keep it reading as an
                action rather than another filter pill. */}
            {isMobile && (
              <div className="ml-auto">
                <CreateIncomeDialog compactTrigger />
              </div>
            )}
          </div>

          {headlineStats.length > 0 && (
            <div className="flex flex-wrap gap-4">
              {headlineStats.map((h) => (
                <Card
                  key={h.currency}
                  className={cn("flex-1 min-w-60", headlineSurface)}
                  data-testid="income-headline"
                >
                  <CardContent className="pt-4 space-y-3">
                    <div>
                      <div className="text-xs uppercase tracking-wide text-muted-foreground">
                        {t("income:headline.total")}
                      </div>
                      <div className="text-2xl font-semibold tabular-nums">
                        {formatCurrency(String(h.total), h.currency)}
                      </div>
                    </div>
                    {/* Regularity / by-person / by-category are peer breakdown
                        blocks: side-by-side columns on desktop (md+), stacked on
                        phones. Regularity is always first, by-person (when a
                        household has more than one earner) second, by-category
                        last. */}
                    <div
                      className={cn(
                        "grid gap-y-4 md:gap-y-0 md:divide-x md:divide-border",
                        "md:[&>*]:px-6 md:[&>*:first-child]:pl-0 md:[&>*:last-child]:pr-0",
                        h.byUser.length > 1 ? "md:grid-cols-3" : "md:grid-cols-2",
                      )}
                    >
                      <div>
                        <div className="text-xs uppercase tracking-wide text-muted-foreground">
                          {t("income:headline.byRegularity")}
                        </div>
                        <dl className="mt-1 space-y-0.5 text-sm">
                          <div className="flex justify-between gap-3">
                            <dt>{t("income:headline.routine")}</dt>
                            <dd className="tabular-nums text-right">
                              {formatCurrency(String(h.routine), h.currency)}
                            </dd>
                          </div>
                          <div className="flex justify-between gap-3">
                            <dt>{t("income:headline.incidental")}</dt>
                            <dd className="tabular-nums text-right">
                              {formatCurrency(String(h.incidental), h.currency)}
                            </dd>
                          </div>
                        </dl>
                      </div>
                      {h.byUser.length > 1 && (
                        <div>
                          <div className="text-xs uppercase tracking-wide text-muted-foreground">
                            {t("income:headline.byPerson")}
                          </div>
                          <dl className="mt-1 space-y-0.5 text-sm">
                            {h.byUser.map((u) => (
                              <div key={u.label} className="flex justify-between gap-3">
                                <dt>{u.label}</dt>
                                <dd className="tabular-nums text-right">
                                  {formatCurrency(String(u.amount), h.currency)}
                                </dd>
                              </div>
                            ))}
                          </dl>
                        </div>
                      )}
                      <div>
                        <div className="text-xs uppercase tracking-wide text-muted-foreground">
                          {t("income:headline.byCategory")}
                        </div>
                        <dl className="mt-1 space-y-0.5 text-sm">
                          {h.byCategory.map((c) => (
                            <div key={c.category} className="flex justify-between gap-3">
                              <dt>{t(`income:categories.${c.category}`)}</dt>
                              <dd className="tabular-nums text-right">
                                {formatCurrency(String(c.amount), h.currency)}
                              </dd>
                            </div>
                          ))}
                        </dl>
                      </div>
                    </div>
                  </CardContent>
                </Card>
              ))}
            </div>
          )}

          {filtered.length === 0 ? (
            <p className="text-sm text-muted-foreground">{t(emptyKey)}</p>
          ) : (
            <IncomeList
              rows={pageRows}
              page={effectivePage}
              totalPages={totalPages}
              onPageChange={setPage}
            />
          )}
        </div>
      )}
    </div>
  );
}
