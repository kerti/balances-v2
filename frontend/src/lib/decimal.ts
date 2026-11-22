// Exact decimal addition on the wire's string decimals (ADR-0011: money is
// DECIMAL(20,4), quantities and rates DECIMAL(20,8)). Floats never touch a
// summed or stored value, so 0.1 + 0.2 is 0.3 and 40 + 10 is "50", not
// "50.00000000000001". Everything is held as a BigInt count of 1e-SCALE units.
const SCALE = 8;
const UNIT = 10n ** BigInt(SCALE);

// "123.45", "-0.5", "7" -> scaled BigInt. Digits beyond SCALE are truncated (the
// backend never stores more); anything that is not a plain decimal returns null.
export function toScaled(value: string): bigint | null {
  const m = /^([+-]?)(\d*)(?:\.(\d*))?$/.exec(value.trim());
  if (!m || (m[2] === "" && (m[3] ?? "") === "")) return null;
  const frac = (m[3] ?? "").padEnd(SCALE, "0").slice(0, SCALE);
  const n = BigInt(m[2] || "0") * UNIT + BigInt(frac);
  return m[1] === "-" ? -n : n;
}

export function fromScaled(n: bigint): string {
  const neg = n < 0n;
  const abs = neg ? -n : n;
  const whole = abs / UNIT;
  const frac = (abs % UNIT).toString().padStart(SCALE, "0").replace(/0+$/, "");
  return `${neg ? "-" : ""}${whole}${frac ? `.${frac}` : ""}`;
}

// Sum of decimal strings; null/undefined/unparseable entries add nothing.
export function sumDecimals(values: Array<string | null | undefined>): string {
  let total = 0n;
  for (const v of values) {
    const n = v == null ? null : toScaled(v);
    if (n !== null) total += n;
  }
  return fromScaled(total);
}

// Exact a − b, or null when either is not a plain decimal.
export function subtractDecimals(a: string, b: string): string | null {
  const x = toScaled(a);
  const y = toScaled(b);
  if (x === null || y === null) return null;
  return fromScaled(x - y);
}

// The amount column's scale, DECIMAL(20,4) (ADR-0011).
const AMOUNT_SCALE = 4;

// Digits and scale of a plain decimal, with no truncation: a product has to be
// formed from every digit before it is rounded.
function parseExact(value: string): { n: bigint; scale: number } | null {
  const m = /^([+-]?)(\d*)(?:\.(\d*))?$/.exec(value.trim());
  if (!m || (m[2] === "" && (m[3] ?? "") === "")) return null;
  const frac = m[3] ?? "";
  const n = BigInt(`${m[2]}${frac}` || "0");
  return { n: m[1] === "-" ? -n : n, scale: frac.length };
}

// Exact a × b, rounded half away from zero to 4 places once, or null when either
// is not a plain decimal. Mirrors the backend's snapshotamount.QuantityPrice
// (a quantity × price_per_unit snapshot's stored amount), so a dialog previews
// the figure the server will store; the two are pinned to the same table.
export function multiplyDecimals(a: string, b: string): string | null {
  const x = parseExact(a);
  const y = parseExact(b);
  if (x === null || y === null) return null;
  const product = x.n * y.n;
  const scale = x.scale + y.scale;
  let rounded: bigint;
  if (scale <= AMOUNT_SCALE) {
    rounded = product * 10n ** BigInt(AMOUNT_SCALE - scale);
  } else {
    const divisor = 10n ** BigInt(scale - AMOUNT_SCALE);
    const abs = product < 0n ? -product : product;
    let q = abs / divisor;
    if ((abs % divisor) * 2n >= divisor) q += 1n;
    rounded = product < 0n ? -q : q;
  }
  return fromScaled(rounded * 10n ** BigInt(SCALE - AMOUNT_SCALE));
}
