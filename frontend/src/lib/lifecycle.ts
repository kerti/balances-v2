// Position lifecycle (ADR-0009). Status enums differ per group; the backend
// (repo validatePositionLifecycle + DB CHECK) is the source of truth, this
// mirror drives the terminate dialog's dropdown and the status badge label.
//
// Labels are resolved through i18next (ADR-0026): callers re-render via a
// useTranslation hook when the locale changes, so the helpers below stay
// pure functions but pick up the live translation each render.
import i18n from "@/i18n";

export type LifecycleGroup = "assets" | "liabilities" | "receivables" | "investments";

export type StatusOption = { value: string; label: string };

// Per-group ordered status values. Source of truth for the dropdown order
// in TerminatePositionDialog and the value-set for statusLabel lookup.
export const STATUS_VALUES: Record<LifecycleGroup, string[]> = {
  assets: ["active", "closed", "sold", "disposed"],
  liabilities: ["active", "paid_off", "forgiven", "written_off"],
  receivables: ["active", "collected", "written_off"],
  investments: ["active", "sold", "matured"],
};

export function statusLabel(group: LifecycleGroup, status: string): string {
  return i18n.t(`common:lifecycle.${group}.${status}`, {
    defaultValue: status,
  });
}

export function statusOptions(group: LifecycleGroup): StatusOption[] {
  return STATUS_VALUES[group].map((value) => ({
    value,
    label: statusLabel(group, value),
  }));
}

export function isActiveStatus(status: string): boolean {
  return status === "active";
}
