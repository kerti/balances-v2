import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { InvestmentPieChart, type PieSlice } from "@/components/InvestmentPieChart";
import { TagBadge } from "@/components/TagBadge";
import { useTags, useTagBreakdown } from "@/hooks/useTags";
import { aggregateTagBreakdown, type TagCell } from "@/lib/tagBreakdown";
import { formatCurrency } from "@/lib/format";

function cellKey(c: TagCell) {
  return c.tagId ?? "untagged";
}

// TagsScreen is the breakdown report (ADR-0028): per currency, a pie of
// holdings proportion by Tag plus a table of holdings / liabilities / net,
// with an Untagged bucket. No FX — a multi-currency household sees one card
// per currency, matching the list/home convention.
export function TagsScreen() {
  const { t } = useTranslation(["tags", "common"]);
  const { data: tags } = useTags();
  const { data: rows, isLoading } = useTagBreakdown();
  const [checked, setChecked] = useState<Record<string, Set<string>>>({});

  const untaggedLabel = t("report.untagged");
  const breakdowns = rows && tags ? aggregateTagBreakdown(rows, tags, untaggedLabel) : [];

  function isChecked(currency: string, key: string) {
    return checked[currency]?.has(key) ?? true;
  }

  function toggle(currency: string, key: string, cells: TagCell[]) {
    setChecked((prev) => {
      const current = prev[currency] ?? new Set(cells.map(cellKey));
      const next = new Set(current);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return { ...prev, [currency]: next };
    });
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">{t("report.title")}</h1>
        <p className="text-sm text-muted-foreground">{t("report.subtitle")}</p>
      </div>

      {!isLoading && breakdowns.length === 0 && (
        <p className="text-sm text-muted-foreground" data-testid="tags-empty">
          {t("report.empty")}
        </p>
      )}

      {breakdowns.map((bd) => {
        const slices: PieSlice[] = bd.cells
          .filter((c) => c.holdings > 0 && isChecked(bd.currency, cellKey(c)))
          .map((c) => ({
            key: cellKey(c),
            label: c.name,
            value: c.holdings,
            color: c.color,
          }));

        return (
          <Card key={bd.currency} data-testid={`tag-breakdown-${bd.currency}`}>
            <CardHeader>
              <CardTitle className="text-base">
                {t("report.currencyHeading", { currency: bd.currency })}
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-6">
              <InvestmentPieChart slices={slices} currency={bd.currency} legendPosition="right" />

              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="w-8" />
                    <TableHead>{t("report.col.tag")}</TableHead>
                    <TableHead className="text-right">{t("report.col.holdings")}</TableHead>
                    <TableHead className="text-right">{t("report.col.liabilities")}</TableHead>
                    <TableHead className="text-right">{t("report.col.net")}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {bd.cells.map((c) => {
                    const key = cellKey(c);
                    const on = isChecked(bd.currency, key);
                    return (
                      <TableRow key={key}>
                        <TableCell>
                          <input
                            type="checkbox"
                            checked={on}
                            onChange={() => toggle(bd.currency, key, bd.cells)}
                            className="h-4 w-4 cursor-pointer accent-primary"
                            aria-label={c.name}
                          />
                        </TableCell>
                        <TableCell>
                          <TagBadge name={c.name} color={c.color} />
                        </TableCell>
                        <TableCell className="text-right tabular-nums">
                          {c.holdings > 0 ? formatCurrency(String(c.holdings), bd.currency) : "—"}
                        </TableCell>
                        <TableCell className="text-right tabular-nums text-destructive">
                          {c.liabilities > 0
                            ? `−${formatCurrency(String(c.liabilities), bd.currency)}`
                            : "—"}
                        </TableCell>
                        <TableCell className="text-right tabular-nums">
                          {formatCurrency(String(c.net), bd.currency)}
                        </TableCell>
                      </TableRow>
                    );
                  })}
                  <TableRow className="font-medium">
                    <TableCell />
                    <TableCell>{t("report.total")}</TableCell>
                    <TableCell className="text-right tabular-nums">
                      {formatCurrency(String(bd.totalHoldings), bd.currency)}
                    </TableCell>
                    <TableCell className="text-right tabular-nums text-destructive">
                      {bd.totalLiabilities > 0
                        ? `−${formatCurrency(String(bd.totalLiabilities), bd.currency)}`
                        : "—"}
                    </TableCell>
                    <TableCell className="text-right tabular-nums">
                      {formatCurrency(String(bd.totalHoldings - bd.totalLiabilities), bd.currency)}
                    </TableCell>
                  </TableRow>
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        );
      })}
    </div>
  );
}
