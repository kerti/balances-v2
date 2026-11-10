import { groupCanonical } from "@/lib/format";

// The desktop **Currency** column names the whole pair — the foreign currency
// being priced and the reporting currency it converts to (ADR-0002: `rate` is
// reporting-currency units per 1 unit of the foreign currency). A bare "USD"
// hides both the base and the direction; "USD → IDR" reads as "USD converts to
// IDR" under a labelled column, with the number safely in its own **Rate**
// column. Falls back to the bare code until the session (so the reporting
// currency) has loaded.
export function fxPair(currency: string, base: string | undefined): string {
  return base ? `${currency} → ${base}` : currency;
}

// The mobile card and the dialog's hint spell the rate as a full **equation**
// (`1 USD = 15600 IDR`) rather than a bare number beside the pair — on a stacked
// card the promoted number would sit flush against "USD → IDR" and misread as
// "15600 USD". The equation binds the number to the *reporting* currency, which
// is what it actually is. `rate` may be "?" in the live hint before a value is
// typed. Built as an expression to stay clear of the ADR-0026 bare-JSX-text
// lint. The rate is grouped the way the rate field shows it (ADR-0056), so the
// hint echoes the figure being typed and a saved row reads the same.
export function fxEquation(currency: string, rate: string, base: string | undefined): string {
  const shown = rate === "?" ? rate : groupCanonical(rate);
  return base ? `1 ${currency} = ${shown} ${base}` : `1 ${currency} = ${shown}`;
}
