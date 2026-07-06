// Display formatting for currencies, dates, year-months, and decimal numbers.
// Locale-aware via i18next's active language (ADR-0026); helpers accept an
// optional locale override for tests and non-React call sites.
//
// IDR (and JPY etc.) display with no decimals; most other currencies with 2.
// The amount comes in as a string for precision; for display we convert
// through Number which is fine at household scale but would need a decimal
// library if we ever did arithmetic on these values in the frontend (don't —
// let the backend compute).
//
// Reactivity note: in React code, the surrounding component should call
// useTranslation() (which subscribes to language changes) so a locale switch
// triggers a rerender and the format helpers below see the new language. Pure
// call sites that read i18n.language directly will not re-evaluate on a
// language change without an outer trigger.
import i18n from "@/i18n";
import { SUPPORTED_LOCALES, type Locale } from "@/i18n";

const NO_DECIMAL_CURRENCIES = new Set(["IDR", "JPY", "KRW", "VND"]);

// Active locale at the moment of the call, as the supported BCP47 string.
// We accept anything i18next hands back (it may be a region-stripped base like
// 'en' or 'id' after load: 'languageOnly' resolution) and project it onto the
// supported set — Intl is fine with either form, but the call surface stays
// regular.
function activeLocale(): Locale {
  const raw = i18n.language || "en-GB";
  if ((SUPPORTED_LOCALES as readonly string[]).includes(raw)) {
    return raw as Locale;
  }
  const base = raw.split("-")[0];
  return base === "id" ? "id-ID" : "en-GB";
}

function resolve(locale: Locale | undefined): Locale {
  return locale ?? activeLocale();
}

export function formatCurrency(amount: string, currency: string, locale?: Locale): string {
  const n = Number(amount);
  if (Number.isNaN(n)) return amount;
  const decimals = NO_DECIMAL_CURRENCIES.has(currency) ? 0 : 2;
  return new Intl.NumberFormat(resolve(locale), {
    style: "currency",
    currency,
    minimumFractionDigits: decimals,
    maximumFractionDigits: decimals,
  }).format(n);
}

// "2024-05-01T00:00:00Z" → en: "May 2024", id: "Mei 2024"
export function formatYearMonth(iso: string, locale?: Locale): string {
  const d = new Date(iso);
  return d.toLocaleDateString(resolve(locale), {
    year: "numeric",
    month: "long",
  });
}

// "2024-05-15T00:00:00Z" → en: "15 May 2024", id: "15 Mei 2024"
export function formatDate(iso: string, locale?: Locale): string {
  return new Date(iso).toLocaleDateString(resolve(locale), {
    day: "numeric",
    month: "short",
    year: "numeric",
  });
}

// "2024-05-15T14:30:00Z" → en: "15 May 2024, 14:30", id: "15 Mei 2024, 14.30"
export function formatDateTime(iso: string, locale?: Locale): string {
  return new Date(iso).toLocaleString(resolve(locale), {
    day: "numeric",
    month: "short",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

// Compact month + 2-digit year for chart axis ticks. en: "May 24",
// id: "Mei 24". Optimised for narrow ticks — use formatYearMonth for body
// copy and formatDate where the day matters.
export function formatChartMonth(d: Date, locale?: Locale): string {
  return d.toLocaleDateString(resolve(locale), {
    month: "short",
    year: "2-digit",
  });
}

// Short month + numeric year. en: "May 2024", id: "Mei 2024". Used by the
// maturity helper for "Matures May 2024" copy where the long month name of
// formatYearMonth would crowd the row.
export function formatShortYearMonth(d: Date, locale?: Locale): string {
  return d.toLocaleDateString(resolve(locale), {
    month: "short",
    year: "numeric",
  });
}

// Locale-aware decimal number, no currency. en: "12,345.67"; id: "12.345,67".
// Used for FX-rate display in the Dashboard.
export function formatNumber(value: number | string, locale?: Locale): string {
  const n = typeof value === "number" ? value : Number(value);
  if (!Number.isFinite(n)) return String(value);
  return new Intl.NumberFormat(resolve(locale)).format(n);
}

// Compact-notation number for chart Y-axis ticks. en: "1.5K", "2.3M";
// id: "1,5 rb", "2,3 jt" (Intl emits the locale-native compact units).
export function formatCompactNumber(value: number, locale?: Locale): string {
  return new Intl.NumberFormat(resolve(locale), {
    notation: "compact",
    maximumFractionDigits: 1,
  }).format(value);
}

// formatSignedPercent renders a signed-decimal rate like property's
// annual_appreciation_rate with an explicit "+" or "−" prefix and 2dp.
// Locale-agnostic — sign + ASCII number only, no thousand separators.
// "3.5"  -> "+3.50%"
// "-2"   -> "−2.00%" (real minus sign, not hyphen)
// "0"    -> "0.00%"
export function formatSignedPercent(value: string): string {
  const n = Number(value);
  if (!Number.isFinite(n)) return value;
  const sign = n > 0 ? "+" : n < 0 ? "−" : "";
  return `${sign}${Math.abs(n).toFixed(2)}%`;
}

// roundToCurrency rounds a decimal-string amount to the precision the currency
// displays at — 0 dp for IDR/JPY/KRW/VND, 2 dp for everything else. Used when
// pasting a computed suggestion (e.g. the revaluation helper) into a snapshot
// amount input so the field shows "19493589" for IDR rather than the helper's
// raw 4dp "19493588.6896". Returns the input unchanged on NaN. Locale-agnostic
// — outputs a raw decimal string, never with thousand separators (this is for
// form input values, not for display).
export function roundToCurrency(amount: string, currency: string): string {
  const n = Number(amount);
  if (!Number.isFinite(n)) return amount;
  const decimals = NO_DECIMAL_CURRENCIES.has(currency) ? 0 : 2;
  const f = Math.pow(10, decimals);
  return (Math.round(n * f) / f).toFixed(decimals);
}
