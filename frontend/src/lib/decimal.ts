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
