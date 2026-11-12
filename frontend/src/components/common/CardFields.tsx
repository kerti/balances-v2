import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

// The mobile card grammar (#849, ADR-0050 "wide table → stacked cards"): a
// label on the left, its value right-aligned, one pair per row, as `IncomeCard`
// established. Every list/history card builds its body from these so the phone
// reads the same everywhere.
export function CardFields({ className, children }: { className?: string; children: ReactNode }) {
  return <dl className={cn("space-y-1 text-sm", className)}>{children}</dl>;
}

export function CardField({
  label,
  children,
  valueTestId,
  className,
}: {
  label: string;
  children: ReactNode;
  valueTestId?: string;
  className?: string;
}) {
  return (
    <div className={cn("flex justify-between gap-3", className)}>
      <dt className="shrink-0 text-muted-foreground">{label}</dt>
      <dd className="min-w-0 text-right" data-testid={valueTestId}>
        {children}
      </dd>
    </div>
  );
}
