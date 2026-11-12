import { statusLabel, type LifecycleGroup } from "@/lib/lifecycle";

// Small inline pill for a position's lifecycle status. Nothing about a status
// is good or bad to own (ADR-0054 amendment 2026-11-09), so Active is a
// neutral outline pill from theme tokens; any terminal status (closed/sold/
// matured/paid off/…) is muted grey: done and inactive, so it recedes and
// pairs with the greyed-out terminated row. List rows omit the pill for
// Active; the detail header shows it for every status.
type Props = {
  group: LifecycleGroup;
  status: string;
};

export function StatusBadge({ group, status }: Props) {
  const active = status === "active";
  const cls = active
    ? "border border-border bg-transparent text-foreground"
    : "bg-muted text-muted-foreground";
  return (
    <span
      data-testid="status-badge"
      className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${cls}`}
    >
      {statusLabel(group, status)}
    </span>
  );
}
