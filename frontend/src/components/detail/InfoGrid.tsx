import { Fragment } from "react";
import type { InfoField } from "@/components/detail/types";

// The shared info-card body (ADR-0051). Lays out label/value pairs and owns the
// responsive reflow (grid → stacked pairs on mobile, Phase B). It **never
// inspects field identity or domain meaning**: alignment/formatting ride on the
// value node itself, and each value cell is a flex container so a per-node
// `ml-auto` / `text-right` lines up a numeric column without the grid learning
// what "numeric" means. Renders nothing for an empty field list (an amount-only
// type like BankAccount, whose info card is title + status line only).
export function InfoGrid({ fields }: { fields: InfoField[] }) {
  if (fields.length === 0) return null;
  return (
    <dl className="grid grid-cols-[auto_1fr] gap-x-6 gap-y-1 text-sm">
      {fields.map((field, i) => (
        <Fragment key={i}>
          <dt className="text-muted-foreground">{field.label}</dt>
          <dd className="flex">{field.value}</dd>
        </Fragment>
      ))}
    </dl>
  );
}
