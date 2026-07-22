import { useTranslation } from "react-i18next";
import { Card, CardContent } from "@/components/ui/card";
import { IncomeRowMenu } from "@/components/income/IncomeRowMenu";
import { useIncomeRow } from "@/components/income/useIncomeRow";
import type { Income } from "@/api/types";

type Props = {
  income: Income;
};

// Mobile leaf renderer (ADR-0050 "wide table → stacked cards"): one card per
// income row, the amount promoted to the card headline so the number the
// household member came for reads with no horizontal scroll, and the remaining
// fields stacked as label→value pairs below. Shares `useIncomeRow` with the
// desktop table row (same projection, same dialogs) and the same `income-*`
// data-testids. The ⋮ trigger sizes up to the 44px tap floor.
export function IncomeCard({ income }: Props) {
  const { t } = useTranslation(["income"]);
  const row = useIncomeRow(income);
  const { RegularityIcon } = row;

  return (
    <>
      <Card data-testid="income-row">
        <CardContent className="space-y-2 p-4">
          {/* Top line: the amount (promoted headline) on the left; the category
              chip + regularity icon float to the right, clustered next to the
              actions button. */}
          <div className="flex items-center gap-2">
            <div className="flex-1 text-xl font-semibold tabular-nums" data-testid="income-amount">
              {row.amountText}
            </div>
            <div className="flex shrink-0 items-center gap-1.5">
              <span className="inline-flex items-center rounded-full border px-2 py-0.5 text-xs">
                {row.categoryChip}
              </span>
              <RegularityIcon
                className="size-3.5 text-muted-foreground"
                aria-label={row.regularityLabel}
                data-testid={`regularity-${row.regularity}`}
              />
            </div>
            <IncomeRowMenu
              onEdit={row.onEdit}
              onDuplicate={row.onDuplicate}
              onDelete={row.onDelete}
              variant="outline"
              triggerClassName="size-11 shrink-0"
            />
          </div>

          {/* The field list spans the full card width, so its values right-align
              to the same p-4 edge the actions button sits against. */}
          <dl className="space-y-1 text-sm">
            <div className="flex justify-between gap-3">
              <dt className="text-muted-foreground">{t("income:tableHeaders.date")}</dt>
              <dd className="whitespace-nowrap text-right">{row.dateText}</dd>
            </div>
            {row.description && (
              <div className="flex justify-between gap-3">
                <dt className="shrink-0 text-muted-foreground">
                  {t("income:tableHeaders.description")}
                </dt>
                <dd className="min-w-0 text-right">{row.description}</dd>
              </div>
            )}
            <div className="flex justify-between gap-3">
              <dt className="text-muted-foreground">{t("income:tableHeaders.ownership")}</dt>
              <dd className="text-right">{row.ownerLabel}</dd>
            </div>
          </dl>
        </CardContent>
      </Card>

      {row.dialogs}
    </>
  );
}
