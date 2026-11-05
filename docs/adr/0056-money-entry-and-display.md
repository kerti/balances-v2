# Money entry and display: live grouping, either separator means decimal, IDR has cents

Found by the 2026-11-04 UI audit (BUG-01, MOB-08). Every amount, quantity, price and rate field sent
whatever the user typed straight to the server. `decimal.Decimal` can't read `48,500,000`, so typing
an amount the way the app displays it failed with `INVALID_JSON_BODY`. The only form that worked was
plain digits with a `.` decimal point, in either locale. This ADR sets one rule for entering numbers
and amends [[adr-0011]]'s display table.

## Decisions

1. **One numeric input, grouped live as you type.** The field shows `48,500,000` (en-GB) or
   `48.500.000` (id-ID) while you type, not only after blur. The model is digits plus at most one
   decimal point, with a maximum number of decimal places per field. The caret stays after the same
   digit across every re-render, backspace over a group separator takes the digit beside it, focus
   puts the caret at the end but leaves a selected range alone, and a paste that would exceed the
   field's limit is refused whole, never truncated. This ports kerti/uruni's `AmountInput` (#749),
   which moved from bare-digits-while-focused to live grouping after its own UX sweep: a long figure
   you can't read back before saving is the error this prevents.
2. **A typed `.` or `,` always means the decimal point.** Because the field inserts the grouping
   itself, a typed separator can only mean a decimal. That covers a phone's decimal keypad, which
   follows the device's region rather than the app's language. On a field with no decimal places
   (JPY, KRW, VND), a typed separator is ignored, so the en-GB habit of typing `48,500,000` costs
   nothing there. On a field with decimals, IDR included, typing that habit key by key gives
   `48.50`; reaffirmed 2026-11-05 (#787) rather than guessing, because the live grouping shows the
   mistake before saving.
3. **Pasted text goes through a strict, locale-aware parser.** It accepts the app locale's grouping
   and decimal separators and strips a currency symbol or spaces, but grouping is valid only in
   correct groups of three. Anything else gets an inline message on the field and never reaches the
   server. The wire format is unchanged: a canonical decimal string such as `48500000.50`.
   Text inserted in one edit rather than keyed (autofill, dictation, text replacement) is read the
   same way when the whole field then parses, so dictating `48,500,000` lands as grouping; a single
   keystroke keeps rule 2. Amended 2026-11-05 (#787), when IDR gaining decimals turned a bulk
   `48,500,000` into two decimal points.
4. **IDR has 2 minor units.** [[adr-0011]] listed "IDR → no decimals" as the ISO 4217 convention, but
   ISO 4217 gives IDR 2, and Indonesian banks state balances in cents. IDR now displays with 2
   decimals everywhere USD and GBP do: inputs, rows, dashboard, heroes, totals, tooltips and the PDF.
   Compact chart-axis labels stay compact for every currency. JPY, KRW and VND stay at 0. Rounding a
   stored IDR value to whole rupiah, for example in the Copy carryover prefill, was silently losing
   the bank's cents.
5. **Numbers use tabular figures by default.** `font-variant-numeric: tabular-nums` is set once on
   `body` instead of on 72 separate elements, so inputs, table cells and cards line up without
   opting in. With 2 decimals and right-aligned columns, the decimal points align. Chart tooltips
   drop `font-mono`.

## Considered options

- **Reformat on blur, raw while typing.** Simpler, with no caret work, but it hides the error live
  grouping prevents: miscounted zeros on large IDR figures. Rejected; Uruni already made this move.
- **Only the locale's decimal key counts; the grouping key is ignored.** It protects the typing habit
  on fields with decimals, but an en-GB app on an Indonesian-region phone could never enter cents.
- **Guess from the digits** (read en-GB `1,5` as 1.5 because the group isn't three digits). It
  leaves `1,500` ambiguous, and money is the wrong place for a silent guess.
- **IDR decimals only where statement figures are entered or checked, whole rupiah in summaries.**
  Less noise on the dashboard, but USD and GBP already show cents everywhere. One rule for every
  currency with minor units beats a per-surface exception.

## Consequences

- `NO_DECIMAL_CURRENCIES` loses IDR. `backend/internal/moneyfmt` needs an explicit override, because
  its CLDR default gives IDR 0 digits. The PDF fixtures get regenerated.
- A large IDR total is 3 characters longer (`Rp1.234.567.890,00`). The dashboard hero and list totals
  need checking at 390px.
