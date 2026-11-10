import { Fragment } from "react";
import { useTranslation } from "react-i18next";
import { ChevronDown, ChevronRight } from "lucide-react";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { TagBadge } from "@/components/common/TagBadge";
import { TagItemsPanel } from "@/components/tags/TagItemsPanel";
import { cellKey, type CurrencyBreakdown } from "@/lib/tagBreakdown";
import { formatCurrency } from "@/lib/format";

type Props = {
  bd: CurrencyBreakdown;
  isChecked: (key: string) => boolean;
  toggle: (key: string) => void;
  isExpanded: (key: string) => boolean;
  toggleExpanded: (key: string) => void;
};

// Desktop leaf renderer (ADR-0050 "wide table → stacked cards", web side): the
// wide holdings / liabilities / net table with a per-row pie-inclusion checkbox
// and a Total footer row. The container (TagsScreen) owns the checked state and
// hands down the same isChecked/toggle the mobile cards use, keyed by cellKey.
export function TagBreakdownTable({ bd, isChecked, toggle, isExpanded, toggleExpanded }: Props) {
  const { t } = useTranslation(["tags"]);

  return (
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
          const on = isChecked(key);
          const open = isExpanded(key);
          const Chevron = open ? ChevronDown : ChevronRight;
          return (
            <Fragment key={key}>
              <TableRow>
                <TableCell>
                  <input
                    type="checkbox"
                    checked={on}
                    onChange={() => toggle(key)}
                    className="h-4 w-4 cursor-pointer accent-primary"
                    aria-label={c.name}
                  />
                </TableCell>
                <TableCell>
                  <button
                    type="button"
                    className="flex items-center gap-1 rounded-md"
                    aria-expanded={open}
                    aria-label={t(open ? "report.collapse" : "report.expand", { name: c.name })}
                    onClick={() => toggleExpanded(key)}
                    data-testid={`tag-expand-${key}-${bd.currency}`}
                  >
                    <Chevron className="size-4 shrink-0" aria-hidden />
                    <TagBadge name={c.name} color={c.color} />
                  </button>
                </TableCell>
                <TableCell className="text-right">
                  {c.holdings > 0 ? formatCurrency(String(c.holdings), bd.currency) : "—"}
                </TableCell>
                <TableCell className="text-right">
                  {c.liabilities > 0
                    ? `−${formatCurrency(String(c.liabilities), bd.currency)}`
                    : "—"}
                </TableCell>
                <TableCell className="text-right">
                  {formatCurrency(String(c.net), bd.currency)}
                </TableCell>
              </TableRow>
              {open && (
                <TableRow>
                  <TableCell />
                  <TableCell colSpan={4} className="whitespace-normal">
                    <TagItemsPanel tagId={c.tagId} tagName={c.name} currency={bd.currency} />
                  </TableCell>
                </TableRow>
              )}
            </Fragment>
          );
        })}
        <TableRow className="font-medium">
          <TableCell />
          <TableCell>{t("report.total")}</TableCell>
          <TableCell className="text-right">
            {formatCurrency(String(bd.totalHoldings), bd.currency)}
          </TableCell>
          <TableCell className="text-right">
            {bd.totalLiabilities > 0
              ? `−${formatCurrency(String(bd.totalLiabilities), bd.currency)}`
              : "—"}
          </TableCell>
          <TableCell className="text-right">
            {formatCurrency(String(bd.totalHoldings - bd.totalLiabilities), bd.currency)}
          </TableCell>
        </TableRow>
      </TableBody>
    </Table>
  );
}
