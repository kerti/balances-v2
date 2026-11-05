// Display formatting for currencies, dates, year-months, and decimal numbers.
// Locale-aware via i18next's active language (ADR-0026); helpers accept an
// optional locale override for tests and non-React call sites.
//
// JPY, KRW and VND display with no decimals; every other currency, IDR included,
// with 2 (ADR-0056 §4: ISO 4217 gives IDR 2 minor units and banks state cents).
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

const NO_DECIMAL_CURRENCIES = new Set(["JPY", "KRW", "VND"]);

// The number of minor units a currency displays and accepts on entry.
export function currencyDecimals(currency: string): number {
  return NO_DECIMAL_CURRENCIES.has(currency) ? 0 : 2;
}

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
  const decimals = currencyDecimals(currency);
  return new Intl.NumberFormat(resolve(locale), {
    style: "currency",
    currency,
    minimumFractionDigits: decimals,
    maximumFractionDigits: decimals,
  }).format(n);
}

// formatCurrencyParts splits a currency render into its symbol and its number
// so a caller can lay them out in separate columns — the symbol left-aligned,
// the number right-aligned — instead of the single ragged string formatCurrency
// returns. Same locale + per-currency decimal rules as formatCurrency. `value`
// is null when the amount isn't a finite number (the caller shows a placeholder);
// `symbol` is always resolved (from a formatted zero) so it can head the column
// even before a value exists.
export function formatCurrencyParts(
  amount: string,
  currency: string,
  locale?: Locale,
): { symbol: string; value: string | null } {
  const decimals = currencyDecimals(currency);
  const fmt = new Intl.NumberFormat(resolve(locale), {
    style: "currency",
    currency,
    minimumFractionDigits: decimals,
    maximumFractionDigits: decimals,
  });
  const symbol = fmt
    .formatToParts(0)
    .filter((p) => p.type === "currency")
    .map((p) => p.value)
    .join("");
  const n = Number(amount);
  if (amount.trim() === "" || Number.isNaN(n)) return { symbol, value: null };
  const value = fmt
    .formatToParts(n)
    .filter((p) => p.type !== "currency" && p.type !== "literal")
    .map((p) => p.value)
    .join("");
  return { symbol, value };
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

// formatPercent renders a decimal-string rate as a plain 2dp percentage — no
// forced "+" on positives (unlike formatSignedPercent, which is for signed
// appreciation figures): normal inflation reads as "3.50%", deflation carries
// the Intl minus ("-2.00%"). Locale-aware separators to match formatNumber (the
// app's numeric display standard) — "3,50%" under id-ID. Used for the manual
// inflation-rate table, where the figures are always shown to a fixed 2dp and
// right-aligned so the decimal points line up.
export function formatPercent(value: number | string, locale?: Locale): string {
  const n = typeof value === "number" ? value : Number(value);
  if (!Number.isFinite(n)) return String(value);
  const num = new Intl.NumberFormat(resolve(locale), {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(n);
  return `${num}%`;
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
  const decimals = currencyDecimals(currency);
  const f = Math.pow(10, decimals);
  return (Math.round(n * f) / f).toFixed(decimals);
}

// The locale's digit-grouping and decimal marks. en-GB: "," and "."; id-ID:
// "." and ",".
export function numberSeparators(locale?: Locale): { group: string; decimal: string } {
  const parts = new Intl.NumberFormat(resolve(locale)).formatToParts(1234.5);
  return {
    group: parts.find((p) => p.type === "group")?.value ?? ",",
    decimal: parts.find((p) => p.type === "decimal")?.value ?? ".",
  };
}

// groupCanonical renders a canonical decimal string ("48500000.50") in the
// locale's marks ("48,500,000.50" / "48.500.000,50") without going through
// Number, so a 20-digit figure is never rounded. A trailing "." stays, so a
// half-typed figure reads back as typed (ADR-0056).
export function groupCanonical(canonical: string, locale?: Locale): string {
  const { group, decimal } = numberSeparators(locale);
  const dot = canonical.indexOf(".");
  const int = dot === -1 ? canonical : canonical.slice(0, dot);
  const grouped = int.replace(/\B(?=(\d{3})+(?!\d))/g, group);
  return dot === -1 ? grouped : grouped + decimal + canonical.slice(dot + 1);
}

function escapeRegExp(s: string): string {
  return s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

// parseLocaleNumber is the strict reader for pasted text (ADR-0056 §3): it
// accepts the locale's grouping and decimal marks, with grouping valid only in
// correct groups of three, and strips a currency symbol or code and spaces.
// Returns the canonical decimal string ("48500000.50"), or null for anything
// else. A leading minus (hyphen or U+2212) is read only with allowNegative.
export function parseLocaleNumber(
  text: string,
  locale?: Locale,
  { allowNegative = false }: { allowNegative?: boolean } = {},
): string | null {
  const { group, decimal } = numberSeparators(locale);
  let s = text
    .replace(/\s/gu, "")
    .replace(/^[\p{L}\p{Sc}]+/u, "")
    .replace(/[\p{L}\p{Sc}]+$/u, "");
  const negative = /^[-\u2212]/.test(s);
  if (negative) {
    if (!allowNegative) return null;
    s = s.slice(1);
  }
  const g = escapeRegExp(group);
  const d = escapeRegExp(decimal);
  const m = new RegExp(`^(\\d{1,3}(?:${g}\\d{3})+|\\d*)(?:${d}(\\d+))?$`).exec(s);
  if (!m || (m[1] === "" && m[2] === undefined)) return null;
  const int =
    m[1]
      .split(group)
      .join("")
      .replace(/^0+(?=\d)/, "") || "0";
  const sign = negative ? "-" : "";
  return m[2] === undefined ? sign + int : `${sign}${int}.${m[2]}`;
}

// Entry limits for the non-money numeric columns (ADR-0011 precision shapes):
// quantities are NUMERIC(20,8); a price per unit is NUMERIC(20,4), and takes all
// four places because a fund's NAV is quoted to four.
export const QUANTITY_DECIMALS = 8;
export const QUANTITY_INTEGER_DIGITS = 12;
export const PRICE_DECIMALS = 4;
// Rates on positions and FX rates are NUMERIC(20,8); inflation rates (monthly
// and the assumed annual one) are NUMERIC(9,4); gold purity is a 0–1 fraction,
// NUMERIC(5,4); a term in months is a whole number.
export const RATE_DECIMALS = 8;
export const RATE_INTEGER_DIGITS = 12;
export const INFLATION_DECIMALS = 4;
export const INFLATION_INTEGER_DIGITS = 5;
export const PURITY_DECIMALS = 4;
export const TERM_MONTHS_DIGITS = 4;
