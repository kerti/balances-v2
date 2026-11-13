import type { EntryRow } from "@/hooks/useBulkEntry";
import { timeDepositInterest } from "@/lib/timeDepositInterest";
import {
  currencyDecimals,
  PRICE_DECIMALS,
  QUANTITY_DECIMALS,
  QUANTITY_INTEGER_DIGITS,
  roundToCurrency,
} from "@/lib/format";

// The bulk monthly-entry screen (ADR-0046) renders one row per eligible
// position, but the *editable* part of a row differs by the position's snapshot
// input shape (ADR-0022): amount-only positions take a single value, while
// qty×price positions (Stock/MutualFund/Gold, #423) take two tab-stops
// (quantity, price per unit) with the value computed. An `EntryShape` captures
// exactly that per-shape variation — which fields to render, how to seed them
// from the row's carry-forward prefill, when the row is complete enough to save,
// and the computed value to display — so the EntryScreen chrome (when-control,
// grouping, dirty-only atomic save, per-row errors) stays shape-agnostic.

// EntryFieldValues holds a row's raw input strings keyed by field key. The keys
// double as the wire field names sent to the bulk endpoint (amount; or
// quantity + price_per_unit), so the save body needs no per-shape remapping.
export type EntryFieldValues = Record<string, string>;

// One editable input in a row.
export type EntryField = {
  // Logical + wire key: "amount" for amount-only; "quantity" / "price_per_unit"
  // for qty×price. Sent verbatim in the bulk save row.
  key: string;
  // data-testid suffix: the input is `${tid}-entry-${testidSuffix}-${id}`. Kept
  // distinct from `key` so amount-only stays `…-entry-amount-…` (the #421/#422
  // testid) while qty×price uses short `quantity` / `price` suffixes.
  testidSuffix: string;
  // i18n key (common namespace) for the field's label/aria-label, or null for
  // the single unlabelled amount-only input (its column header is implicit).
  labelKey: string | null;
  // Tailwind width for the input, tuned per shape (one wide field vs two narrow).
  widthClass: string;
  // Entry limits for the field's NumericInput (ADR-0056): decimal places for a
  // row in `currency`, and the integer digits its column holds (16 when unset).
  decimals: (currency: string) => number;
  maxIntegerDigits?: number;
  // True for a money field (an amount, accrued interest): a carried-forward value
  // is *shown* at the currency's minor units, "1408210.1634" as "1,408,210.16"
  // (ADR-0056). Never set on a quantity or a price per unit, which legitimately
  // carry more places than money does. Display only: dirty-tracking and the
  // save body read the stored value, so an untouched row writes nothing.
  money?: boolean;
};

export type EntryShape = {
  fields: EntryField[];
  // The carry-forward prefill per field key ("" when the row has no history).
  prefill: (row: EntryRow) => EntryFieldValues;
  // Every required field is present and numeric — a row that isn't complete
  // can't be dirty (you can't save half a qty×price pair).
  complete: (v: EntryFieldValues) => boolean;
  // The computed value to display (qty×price: quantity × price), or null for a
  // shape with nothing to compute (amount-only). `valid` is false while the
  // inputs don't yet form a number, so the screen shows a placeholder. `below`
  // (time deposit: Value under the principal) raises the row's soft warning.
  derived: ((v: EntryFieldValues) => { valid: boolean; amount: string; below?: boolean }) | null;
  // The month's value as the "Current: …" line shows it, when it isn't the
  // derived figure (a time deposit derives interest but its value is the field).
  current?: (v: EntryFieldValues) => string;
  // i18n key naming the derived column ("Value" for qty×price, "Principal" for
  // accrued), or null for a shape with no derived column. Desktop positions the
  // total under an (unlabelled) column slot; the mobile stack has no such slot,
  // so EntryRowMobile renders this as the footer's header — the mobile twin of
  // the field labels, so the computed figure reads as a named column not a stray
  // number. Always set when `derived` is non-null.
  derivedLabelKey: string | null;
};

// minorUnitsShown is how a carried-forward money value reads in its input: the
// stored figure rounded to the currency's minor units, without padding zeros it
// didn't have ("1408210.1634" -> "1408210.16", "12500000" stays "12500000"). Not
// padding keeps the field reading like what the user would type (NumericInput
// shows no forced cents) and keeps a click-and-type from landing past the
// field's decimal limit. Display only: the stored string stays the dirty-check
// baseline and the save value, so rounding here never reaches the server.
export function minorUnitsShown(stored: string, currency: string): string {
  if (stored.trim() === "") return stored;
  const rounded = roundToCurrency(stored, currency);
  return rounded.includes(".") ? rounded.replace(/\.?0+$/, "") : rounded;
}

// deriveProduct computes quantity × price with Number — household scale is fine
// and the precision-sensitive arithmetic is re-done on the backend
// (decimal.Decimal), which also derives the stored amount rather than trusting
// this. Mirrors CreateQuantityPriceSnapshotDialog.deriveAmount.
function deriveProduct(quantity: string, price: string): { valid: boolean; amount: string } {
  const q = Number(quantity);
  const p = Number(price);
  if (!quantity || !price || Number.isNaN(q) || Number.isNaN(p)) {
    return { valid: false, amount: "" };
  }
  return { valid: true, amount: (q * p).toString() };
}

// isNumeric reports whether a raw input string is a present, finite number —
// the accrued shape's completeness test for each of its two directly-entered
// fields (total value, accrued interest).
function isNumeric(s: string): boolean {
  return s.trim() !== "" && !Number.isNaN(Number(s));
}

// derivePrincipal computes total value − accrued interest, the "of which
// principal" figure the per-position accrued dialog shows. Mirrors
// CreateAccruedInterestSnapshotDialog.derivePrincipal, surfaced here as the
// accrued shape's derived display column.
function derivePrincipal(amount: string, accrued: string): { valid: boolean; amount: string } {
  const a = Number(amount);
  const i = Number(accrued);
  if (!amount || !accrued || Number.isNaN(a) || Number.isNaN(i)) {
    return { valid: false, amount: "" };
  }
  return { valid: true, amount: (a - i).toString() };
}

// amount-only (Asset/Liability/Receivable): one value field, no computed line.
export const amountOnlyShape: EntryShape = {
  fields: [
    {
      key: "amount",
      testidSuffix: "amount",
      labelKey: null,
      widthClass: "w-36",
      decimals: currencyDecimals,
      money: true,
    },
  ],
  prefill: (row) => ({ amount: row.prefill_amount ?? "" }),
  complete: (v) => v.amount.trim() !== "",
  derived: null,
  derivedLabelKey: null,
};

// qty×price (Stock/MutualFund/Gold, #423): two tab-stops with the value computed
// as quantity × price. Wire keys match the bulk endpoint's expected columns.
export const qtyPriceShape: EntryShape = {
  fields: [
    {
      key: "quantity",
      testidSuffix: "quantity",
      labelKey: "bulkEntry.quantity",
      widthClass: "w-28",
      decimals: () => QUANTITY_DECIMALS,
      maxIntegerDigits: QUANTITY_INTEGER_DIGITS,
    },
    {
      key: "price_per_unit",
      testidSuffix: "price",
      labelKey: "bulkEntry.pricePerUnit",
      widthClass: "w-28",
      decimals: () => PRICE_DECIMALS,
    },
  ],
  prefill: (row) => ({
    quantity: row.prefill_quantity ?? "",
    price_per_unit: row.prefill_price ?? "",
  }),
  complete: (v) => deriveProduct(v.quantity, v.price_per_unit).valid,
  derived: (v) => deriveProduct(v.quantity, v.price_per_unit),
  derivedLabelKey: "bulkEntry.value",
};

// accrued (Bond/TimeDeposit, #424): two directly-entered tab-stops — the total
// value (stored as `amount`) and the accrued-interest component — the accrued
// branch of investment_snapshot_shape. Unlike qty×price, `amount` is entered,
// not computed (a bond's total value already *is* its snapshot amount); the
// derived column shows the principal (total − accrued), mirroring the
// per-position accrued dialog's "of which principal" line. When a row has no
// history, the accrued field's default follows coupon disposition — empty for an
// `accrues` bond (force a real entry, #66) and 0 otherwise (pays-out bonds and
// time deposits, whose coupon lands in the bank each period).
export const accruedShape: EntryShape = {
  fields: [
    {
      key: "amount",
      testidSuffix: "amount",
      labelKey: "bulkEntry.totalValue",
      widthClass: "w-28",
      decimals: currencyDecimals,
      money: true,
    },
    {
      key: "accrued_interest",
      testidSuffix: "accrued",
      labelKey: "bulkEntry.accrued",
      widthClass: "w-28",
      decimals: currencyDecimals,
      money: true,
    },
  ],
  prefill: (row) => ({
    amount: row.prefill_amount ?? "",
    accrued_interest:
      row.prefill_accrued_interest ?? (row.coupon_disposition === "accrues" ? "" : "0"),
  }),
  complete: (v) => isNumeric(v.amount) && isNumeric(v.accrued_interest),
  derived: (v) => derivePrincipal(v.amount, v.accrued_interest),
  derivedLabelKey: "bulkEntry.principal",
  // "Current" is the total value typed last time, not the derived principal (#884).
  current: (v) => v.amount,
};

// time deposit (#844): the one net figure the statement shows (stored as
// `amount`) and nothing else typed; the server derives accrued_interest as
// Value − principal in force. The derived column is that interest, computed live
// from the row's principal (the entry list carries it), and a Value below the
// principal raises a soft warning on the row that never blocks Save. A factory,
// not a constant, because the derivation closes over the row's principal.
export function timeDepositShape(principal: string | null): EntryShape {
  return {
    fields: [
      {
        key: "amount",
        testidSuffix: "amount",
        labelKey: "bulkEntry.value",
        widthClass: "w-28",
        decimals: currencyDecimals,
        money: true,
      },
    ],
    prefill: (row) => ({ amount: row.prefill_amount ?? "" }),
    complete: (v) => isNumeric(v.amount),
    derived: (v) => timeDepositInterest(v.amount, principal),
    derivedLabelKey: "bulkEntry.interest",
    current: (v) => v.amount,
  };
}
