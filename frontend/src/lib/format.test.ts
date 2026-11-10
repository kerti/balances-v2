import { describe, it, expect } from "vitest";
import {
  formatCurrency,
  formatCurrencyParts,
  formatYearMonth,
  formatDate,
  formatDateTime,
  formatChartMonth,
  formatShortMonth,
  formatShortYearMonth,
  formatNumber,
  formatCompactNumber,
  formatSignedPercent,
  formatPercent,
  roundToCurrency,
  numberSeparators,
  groupCanonical,
  parseLocaleNumber,
  currencyDecimals,
} from "@/lib/format";

// Intl output is locale/ICU-dependent (currency glyph placement, spacing,
// digit grouping all vary by Node build), so most assertions check structure —
// the currency code/symbol is present, the right number of decimals appear,
// the month/year/day land — rather than pinning exact glyphs.
//
// Each helper takes an optional `locale` parameter; the tests pass it
// explicitly so the suite doesn't depend on whatever i18n.language happens to
// be when vitest runs.

// covers: INV-PRESENTATION-01
describe("formatCurrency", () => {
  // ADR-0056 §4: IDR has 2 minor units, so a bank's cents survive display.
  it("renders IDR with its cents in both locales", () => {
    const space = /\s/gu;
    expect(formatCurrency("48056131.79", "IDR", "en-GB").replace(space, " ")).toBe(
      "IDR 48,056,131.79",
    );
    expect(formatCurrency("48056131.79", "IDR", "id-ID").replace(space, " ")).toBe(
      "Rp 48.056.131,79",
    );
    expect(formatCurrency("1500000", "IDR", "en-GB")).toMatch(/1,500,000\.00$/);
  });

  it("renders other no-decimal currencies (JPY, KRW, VND) without decimals", () => {
    for (const locale of ["en-GB", "id-ID"] as const) {
      for (const ccy of ["JPY", "KRW", "VND"]) {
        expect(formatCurrency("1000", ccy, locale)).not.toMatch(/[.,]\d{2}\b/);
      }
    }
  });

  it("renders two decimals for ordinary currencies (USD) in both locales", () => {
    for (const locale of ["en-GB", "id-ID"] as const) {
      const out = formatCurrency("1234.5", "USD", locale);
      expect(out).toMatch(/\d/);
      expect(out).toMatch(/[.,]\d{2}\b/);
    }
  });

  it("returns the raw input when the amount is not a number", () => {
    expect(formatCurrency("not-a-number", "USD", "en-GB")).toBe("not-a-number");
  });
});

// covers: INV-PRESENTATION-01
describe("formatCurrencyParts", () => {
  it("splits symbol from number, applying the currency's decimals", () => {
    const idr = formatCurrencyParts("1500000", "IDR", "en-GB");
    expect(idr.symbol).toMatch(/IDR|Rp/);
    expect(idr.value).toBe("1,500,000.00");

    const jpy = formatCurrencyParts("1500000", "JPY", "en-GB");
    expect(jpy.value).toBe("1,500,000"); // JPY: no fraction

    const usd = formatCurrencyParts("1234.5", "USD", "en-GB");
    expect(usd.symbol).toMatch(/\$|USD/);
    expect(usd.value).toMatch(/[.,]\d{2}\b/); // USD: two decimals
  });

  it("still resolves the symbol but yields a null value for a blank/NaN amount", () => {
    for (const bad of ["", "   ", "not-a-number"]) {
      const p = formatCurrencyParts(bad, "IDR", "en-GB");
      expect(p.symbol).toMatch(/IDR|Rp/);
      expect(p.value).toBeNull();
    }
  });
});

// Use a midday UTC timestamp so the displayed calendar day doesn't roll under
// a negative-offset runner TZ.
describe("formatYearMonth", () => {
  it("en renders long month + year", () => {
    expect(formatYearMonth("2024-05-15T12:00:00Z", "en-GB")).toBe("May 2024");
  });
  it("id renders the Indonesian month name", () => {
    expect(formatYearMonth("2024-05-15T12:00:00Z", "id-ID")).toBe("Mei 2024");
  });
});

describe("formatDate", () => {
  it("en renders day + short month + year", () => {
    expect(formatDate("2024-05-15T12:00:00Z", "en-GB")).toBe("15 May 2024");
  });
  it("id renders the date with the Indonesian month name", () => {
    expect(formatDate("2024-05-15T12:00:00Z", "id-ID")).toBe("15 Mei 2024");
  });
});

describe("formatDateTime", () => {
  it("en includes day, month, year, hour, minute", () => {
    const out = formatDateTime("2024-05-15T12:00:00Z", "en-GB");
    expect(out).toMatch(/May/);
    expect(out).toMatch(/2024/);
    expect(out).toMatch(/15/);
    expect(out).toMatch(/\d{2}[:.]\d{2}/); // hh:mm or hh.mm
  });
  it("id uses the Indonesian month name", () => {
    const out = formatDateTime("2024-05-15T12:00:00Z", "id-ID");
    expect(out).toMatch(/Mei/);
  });
});

describe("formatShortYearMonth", () => {
  it("en renders short month + full year", () => {
    expect(formatShortYearMonth(new Date("2024-05-15T12:00:00Z"), "en-GB")).toBe("May 2024");
  });
  it("id renders the Indonesian short month + full year", () => {
    expect(formatShortYearMonth(new Date("2024-05-15T12:00:00Z"), "id-ID")).toBe("Mei 2024");
  });
});

describe("formatShortMonth", () => {
  it("renders September as Sep in en, never Sept", () => {
    const d = new Date("2024-09-15T12:00:00Z");
    expect(formatShortMonth(d, "en-GB")).toBe("Sep");
    expect(formatShortMonth(d, "en-GB", "numeric")).toBe("Sep 2024");
    expect(formatChartMonth(d, "en-GB")).toBe("Sep 24");
  });
  it("leaves Bahasa to Intl", () => {
    expect(formatShortMonth(new Date("2024-05-15T12:00:00Z"), "id-ID")).toBe("Mei");
  });
});

describe("formatChartMonth", () => {
  it("en renders short month + 2-digit year", () => {
    expect(formatChartMonth(new Date("2024-05-15T12:00:00Z"), "en-GB")).toBe("May 24");
  });
  it("id renders the Indonesian short month + 2-digit year", () => {
    expect(formatChartMonth(new Date("2024-05-15T12:00:00Z"), "id-ID")).toBe("Mei 24");
  });
});

describe("formatCompactNumber", () => {
  it("en uses K/M suffixes", () => {
    // ICU compact-suffix case differs between en-US/en-GB and across Node
    // builds (macOS uppercase, some Linux Node builds lowercase), so the
    // assertion is case-insensitive.
    expect(formatCompactNumber(1500, "en-GB")).toMatch(/1\.5\s*k/i);
    expect(formatCompactNumber(2_300_000, "en-GB")).toMatch(/2\.3\s*m/i);
  });
  it("id uses locale-native compact units", () => {
    // id-ID compact uses "rb" (ribu) and "jt" (juta) — assert presence of
    // the unit rather than exact spacing/glyph.
    expect(formatCompactNumber(1500, "id-ID")).toMatch(/rb/);
    expect(formatCompactNumber(2_300_000, "id-ID")).toMatch(/jt/);
  });
});

describe("formatNumber", () => {
  it("en uses comma thousands and dot decimal", () => {
    // en-GB grouping uses commas: "12,345.67"
    expect(formatNumber(12345.67, "en-GB")).toBe("12,345.67");
  });
  it("id uses dot thousands and comma decimal", () => {
    // id-ID grouping uses dots: "12.345,67"
    expect(formatNumber(12345.67, "id-ID")).toBe("12.345,67");
  });
  it("accepts string input", () => {
    expect(formatNumber("1000", "en-GB")).toBe("1,000");
  });
  it("returns the raw string when the value is not a number", () => {
    expect(formatNumber("not-a-number", "en-GB")).toBe("not-a-number");
  });
});

describe("formatSignedPercent", () => {
  it('prefixes positive with "+" and 2dp', () => {
    expect(formatSignedPercent("3.5")).toBe("+3.50%");
  });
  it('prefixes negative with real minus "−" and 2dp', () => {
    expect(formatSignedPercent("-2")).toBe("−2.00%");
  });
  it("renders zero without a sign", () => {
    expect(formatSignedPercent("0")).toBe("0.00%");
  });
  it("returns the raw input when the value is not a number", () => {
    expect(formatSignedPercent("abc")).toBe("abc");
  });
});

describe("formatPercent", () => {
  it("renders a positive rate to a fixed 2dp with no forced sign", () => {
    expect(formatPercent("3.5", "en-GB")).toBe("3.50%");
  });
  it("accepts a numeric value, not only a decimal string", () => {
    expect(formatPercent(3.5, "en-GB")).toBe("3.50%");
  });
  it("pads a whole number to 2dp", () => {
    expect(formatPercent("4", "en-GB")).toBe("4.00%");
  });
  it("keeps the minus on a negative (deflation) rate", () => {
    expect(formatPercent("-2", "en-GB")).toBe("-2.00%");
  });
  it("uses the locale decimal separator (id-ID → comma)", () => {
    expect(formatPercent("3.5", "id-ID")).toBe("3,50%");
  });
  it("returns the raw input when the value is not a number", () => {
    expect(formatPercent("abc", "en-GB")).toBe("abc");
  });
});

describe("roundToCurrency", () => {
  // Rounding a stored IDR value to whole rupiah silently lost the bank's cents
  // (the revaluation Apply prefill turned 48056131.79 into 48056132).
  it("keeps IDR cents, rounding to 2 dp", () => {
    expect(roundToCurrency("48056131.79", "IDR")).toBe("48056131.79");
    expect(roundToCurrency("19493588.6896", "IDR")).toBe("19493588.69");
  });
  it("rounds to 0 dp for other no-decimal currencies (JPY/KRW/VND)", () => {
    for (const ccy of ["JPY", "KRW", "VND"]) {
      expect(roundToCurrency("1234.567", ccy)).toBe("1235");
    }
  });
  it("rounds to 2 dp for ordinary currencies (USD)", () => {
    expect(roundToCurrency("19493588.6896", "USD")).toBe("19493588.69");
    expect(roundToCurrency("100", "USD")).toBe("100.00");
  });
  it("returns the raw input when the amount is not a number", () => {
    expect(roundToCurrency("not-a-number", "IDR")).toBe("not-a-number");
  });
});

// covers: INV-PRESENTATION-13
describe("numberSeparators", () => {
  it("reads the grouping and decimal marks from the locale", () => {
    expect(numberSeparators("en-GB")).toEqual({ group: ",", decimal: "." });
    expect(numberSeparators("id-ID")).toEqual({ group: ".", decimal: "," });
  });
});

describe("groupCanonical", () => {
  it("groups the integer part and swaps in the locale's decimal mark", () => {
    expect(groupCanonical("48500000.50", "en-GB")).toBe("48,500,000.50");
    expect(groupCanonical("48500000.50", "id-ID")).toBe("48.500.000,50");
  });

  it("keeps a trailing decimal point, so a half-typed figure reads as typed", () => {
    expect(groupCanonical("1234.", "en-GB")).toBe("1,234.");
  });

  it("does not round a figure past what a float holds", () => {
    expect(groupCanonical("1234567890123456.1234", "en-GB")).toBe("1,234,567,890,123,456.1234");
  });

  it("groups a signed figure after its minus", () => {
    expect(groupCanonical("-1234567.5", "en-GB")).toBe("-1,234,567.5");
    expect(groupCanonical("-123", "en-GB")).toBe("-123");
  });

  it("renders empty as empty", () => {
    expect(groupCanonical("", "en-GB")).toBe("");
  });
});

describe("parseLocaleNumber", () => {
  it("reads the locale's own grouped form", () => {
    expect(parseLocaleNumber("48,500,000.50", "en-GB")).toBe("48500000.50");
    expect(parseLocaleNumber("48.500.000,50", "id-ID")).toBe("48500000.50");
  });

  it("reads plain digits with the locale's decimal mark", () => {
    expect(parseLocaleNumber("1234.5", "en-GB")).toBe("1234.5");
    expect(parseLocaleNumber("1234,5", "id-ID")).toBe("1234.5");
    expect(parseLocaleNumber("1,5", "id-ID")).toBe("1.5");
  });

  it("strips a currency symbol or code and spaces", () => {
    expect(parseLocaleNumber("Rp 48.500.000", "id-ID")).toBe("48500000");
    expect(parseLocaleNumber("Rp 48.500.000,00", "id-ID")).toBe("48500000.00");
    expect(parseLocaleNumber("£1,234.56", "en-GB")).toBe("1234.56");
    expect(parseLocaleNumber(" 1,234 IDR ", "en-GB")).toBe("1234");
  });

  it("accepts grouping only in correct groups of three", () => {
    expect(parseLocaleNumber("1,5", "en-GB")).toBeNull();
    expect(parseLocaleNumber("12,34,567", "en-GB")).toBeNull();
    expect(parseLocaleNumber("1,2345", "en-GB")).toBeNull();
    expect(parseLocaleNumber("1.000,5", "en-GB")).toBeNull();
  });

  it("refuses anything else", () => {
    expect(parseLocaleNumber("", "en-GB")).toBeNull();
    expect(parseLocaleNumber("abc", "en-GB")).toBeNull();
    expect(parseLocaleNumber("-500", "en-GB")).toBeNull();
    expect(parseLocaleNumber("1.2.3", "en-GB")).toBeNull();
    expect(parseLocaleNumber("12a34", "en-GB")).toBeNull();
  });

  it("reads a leading minus (hyphen or U+2212) only when asked", () => {
    expect(parseLocaleNumber("-1,500.5", "en-GB", { allowNegative: true })).toBe("-1500.5");
    expect(parseLocaleNumber("−3,5", "id-ID", { allowNegative: true })).toBe("-3.5");
    expect(parseLocaleNumber("Rp -1.000", "id-ID", { allowNegative: true })).toBe("-1000");
    expect(parseLocaleNumber("-1,500", "en-GB")).toBeNull();
    expect(parseLocaleNumber("1-500", "en-GB", { allowNegative: true })).toBeNull();
  });

  it("drops leading zeros and reads a bare fraction", () => {
    expect(parseLocaleNumber("007", "en-GB")).toBe("7");
    expect(parseLocaleNumber(".5", "en-GB")).toBe("0.5");
  });
});

describe("currencyDecimals", () => {
  it("gives the currency's minor units", () => {
    expect(currencyDecimals("USD")).toBe(2);
    expect(currencyDecimals("IDR")).toBe(2);
    expect(currencyDecimals("JPY")).toBe(0);
  });
});
