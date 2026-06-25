import { statusLabel, type LifecycleGroup } from "@/lib/lifecycle";

// Small inline pill for a position's lifecycle status. Active is green (the
// live state — reads as "on" at a glance); any terminal status (closed/sold/
// matured/paid off/…) is muted grey: done and inactive, so it recedes and
// pairs with the greyed-out terminated row rather than firing a false alarm.
type Props = {
  group: LifecycleGroup;
  status: string;
};

export function StatusBadge({ group, status }: Props) {
  const active = status === "active";
  const cls = active
    ? "bg-green-100 text-green-800"
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
