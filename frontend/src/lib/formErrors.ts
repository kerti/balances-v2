// Pure helpers behind ValidatedForm (#824, ADR-0058 §5): which form field a
// server error code belongs to, and the localised copy for a native constraint
// failure. The wire contract is unchanged (ADR-0027); this only decides where a
// code is drawn.
import i18n from "@/i18n";
import { ApiError, isEnvelope } from "@/api/client";
import { formatDate } from "@/lib/format";

// Server error code -> the field it is about, as an id *suffix*: a field matches
// when its id equals the suffix or ends with it ("year_month" matches
// `year_month`, `inv_year_month` and `ai_year_month`). Only codes whose field
// exists in every dialog that can raise them belong here; anything else, and
// every VALIDATION envelope (its message already names the field), stays a
// form-level message.
export const FIELD_FOR_CODE: Record<string, string> = {
  SNAPSHOT_MONTH_EXISTS: "year_month",
  FUTURE_YEAR_MONTH: "year_month",
  SNAPSHOT_FUTURE_DATE: "as_of_date",
  SNAPSHOT_DATE_OUTSIDE_MONTH: "as_of_date",
  TRANSACTION_FUTURE_DATE: "_date",
  INVALID_DEPOSIT_TERM: "maturity_date",
  FX_RATE_EXISTS: "month",
  INFLATION_RATE_EXISTS: "month",
  INVALID_RATE: "rate",
  TAG_NAME_EXISTS: "name",
  CANNOT_INVITE_SELF: "email",
};

export function fieldForError(err: unknown): string | undefined {
  if (!(err instanceof ApiError) || !isEnvelope(err.body)) return undefined;
  return FIELD_FOR_CODE[err.body.code];
}

export function idMatchesField(id: string, field: string): boolean {
  return id === field || id.endsWith(field);
}

type Control = HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement;

export function isControl(el: unknown): el is Control {
  return (
    el instanceof HTMLInputElement ||
    el instanceof HTMLSelectElement ||
    el instanceof HTMLTextAreaElement
  );
}

// The message for a control failing its native constraints (required, min, max,
// pattern, type), or undefined when it is valid. The browser's own bubble text
// is never used: it is in the browser's language, not the app's.
export function nativeMessage(el: Control): string | undefined {
  const v = el.validity;
  if (v.valid) return undefined;
  if (v.valueMissing) return i18n.t("common:validation.required");
  const isDate = el instanceof HTMLInputElement && ["date", "month"].includes(el.type);
  if (v.rangeUnderflow && "min" in el) {
    return isDate
      ? i18n.t("common:validation.dateMin", {
          value: formatDate(`${el.min.slice(0, 10)}T12:00:00Z`),
        })
      : i18n.t("common:validation.numMin", { value: el.min });
  }
  if (v.rangeOverflow && "max" in el) {
    return isDate
      ? i18n.t("common:validation.dateMax", {
          value: formatDate(`${el.max.slice(0, 10)}T12:00:00Z`),
        })
      : i18n.t("common:validation.numMax", { value: el.max });
  }
  return i18n.t("common:validation.invalid");
}
