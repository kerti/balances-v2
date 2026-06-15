import { describe, it, expect } from 'vitest'
import {
  formatCurrency,
  formatYearMonth,
  formatDate,
  formatDateTime,
  formatChartMonth,
  formatShortYearMonth,
  formatNumber,
  formatCompactNumber,
  formatSignedPercent,
  roundToCurrency,
} from '@/lib/format'

// Intl output is locale/ICU-dependent (currency glyph placement, spacing,
// digit grouping all vary by Node build), so most assertions check structure —
// the currency code/symbol is present, the right number of decimals appear,
// the month/year/day land — rather than pinning exact glyphs.
//
// Each helper takes an optional `locale` parameter; the tests pass it
// explicitly so the suite doesn't depend on whatever i18n.language happens to
// be when vitest runs.

// covers: INV-PRESENTATION-01
describe('formatCurrency', () => {
  it('renders no-decimal currencies (IDR) without a fractional part in en', () => {
    const out = formatCurrency('1500000', 'IDR', 'en-GB')
    expect(out).toMatch(/IDR|Rp/)
    expect(out).not.toMatch(/[.,]\d{2}\b/) // no two-digit fraction
  })

  it('renders no-decimal currencies (IDR) without a fractional part in id', () => {
    const out = formatCurrency('1500000', 'IDR', 'id-ID')
    expect(out).toMatch(/Rp/)
    expect(out).not.toMatch(/[.,]\d{2}\b/)
  })

  it('renders other no-decimal currencies (JPY, KRW, VND) without decimals', () => {
    for (const locale of ['en-GB', 'id-ID'] as const) {
      for (const ccy of ['JPY', 'KRW', 'VND']) {
        expect(formatCurrency('1000', ccy, locale)).not.toMatch(/[.,]\d{2}\b/)
      }
    }
  })

  it('renders two decimals for ordinary currencies (USD) in both locales', () => {
    for (const locale of ['en-GB', 'id-ID'] as const) {
      const out = formatCurrency('1234.5', 'USD', locale)
      expect(out).toMatch(/\d/)
      expect(out).toMatch(/[.,]\d{2}\b/)
    }
  })

  it('returns the raw input when the amount is not a number', () => {
    expect(formatCurrency('not-a-number', 'USD', 'en-GB')).toBe('not-a-number')
  })
})

// Use a midday UTC timestamp so the displayed calendar day doesn't roll under
// a negative-offset runner TZ.
describe('formatYearMonth', () => {
  it('en renders long month + year', () => {
    expect(formatYearMonth('2024-05-15T12:00:00Z', 'en-GB')).toBe('May 2024')
  })
  it('id renders the Indonesian month name', () => {
    expect(formatYearMonth('2024-05-15T12:00:00Z', 'id-ID')).toBe('Mei 2024')
  })
})

describe('formatDate', () => {
  it('en renders day + short month + year', () => {
    expect(formatDate('2024-05-15T12:00:00Z', 'en-GB')).toBe('15 May 2024')
  })
  it('id renders the date with the Indonesian month name', () => {
    expect(formatDate('2024-05-15T12:00:00Z', 'id-ID')).toBe('15 Mei 2024')
  })
})

describe('formatDateTime', () => {
  it('en includes day, month, year, hour, minute', () => {
    const out = formatDateTime('2024-05-15T12:00:00Z', 'en-GB')
    expect(out).toMatch(/May/)
    expect(out).toMatch(/2024/)
    expect(out).toMatch(/15/)
    expect(out).toMatch(/\d{2}[:.]\d{2}/) // hh:mm or hh.mm
  })
  it('id uses the Indonesian month name', () => {
    const out = formatDateTime('2024-05-15T12:00:00Z', 'id-ID')
    expect(out).toMatch(/Mei/)
  })
})

describe('formatShortYearMonth', () => {
  it('en renders short month + full year', () => {
    expect(formatShortYearMonth(new Date('2024-05-15T12:00:00Z'), 'en-GB')).toBe(
      'May 2024',
    )
  })
  it('id renders the Indonesian short month + full year', () => {
    expect(formatShortYearMonth(new Date('2024-05-15T12:00:00Z'), 'id-ID')).toBe(
      'Mei 2024',
    )
  })
})

describe('formatChartMonth', () => {
  it('en renders short month + 2-digit year', () => {
    expect(formatChartMonth(new Date('2024-05-15T12:00:00Z'), 'en-GB')).toBe(
      'May 24',
    )
  })
  it('id renders the Indonesian short month + 2-digit year', () => {
    expect(formatChartMonth(new Date('2024-05-15T12:00:00Z'), 'id-ID')).toBe(
      'Mei 24',
    )
  })
})

describe('formatCompactNumber', () => {
  it('en uses K/M suffixes', () => {
    // ICU compact-suffix case differs between en-US/en-GB and across Node
    // builds (macOS uppercase, some Linux Node builds lowercase), so the
    // assertion is case-insensitive.
    expect(formatCompactNumber(1500, 'en-GB')).toMatch(/1\.5\s*k/i)
    expect(formatCompactNumber(2_300_000, 'en-GB')).toMatch(/2\.3\s*m/i)
  })
  it('id uses locale-native compact units', () => {
    // id-ID compact uses "rb" (ribu) and "jt" (juta) — assert presence of
    // the unit rather than exact spacing/glyph.
    expect(formatCompactNumber(1500, 'id-ID')).toMatch(/rb/)
    expect(formatCompactNumber(2_300_000, 'id-ID')).toMatch(/jt/)
  })
})

describe('formatNumber', () => {
  it('en uses comma thousands and dot decimal', () => {
    // en-GB grouping uses commas: "12,345.67"
    expect(formatNumber(12345.67, 'en-GB')).toBe('12,345.67')
  })
  it('id uses dot thousands and comma decimal', () => {
    // id-ID grouping uses dots: "12.345,67"
    expect(formatNumber(12345.67, 'id-ID')).toBe('12.345,67')
  })
  it('accepts string input', () => {
    expect(formatNumber('1000', 'en-GB')).toBe('1,000')
  })
  it('returns the raw string when the value is not a number', () => {
    expect(formatNumber('not-a-number', 'en-GB')).toBe('not-a-number')
  })
})

describe('formatSignedPercent', () => {
  it('prefixes positive with "+" and 2dp', () => {
    expect(formatSignedPercent('3.5')).toBe('+3.50%')
  })
  it('prefixes negative with real minus "−" and 2dp', () => {
    expect(formatSignedPercent('-2')).toBe('−2.00%')
  })
  it('renders zero without a sign', () => {
    expect(formatSignedPercent('0')).toBe('0.00%')
  })
  it('returns the raw input when the value is not a number', () => {
    expect(formatSignedPercent('abc')).toBe('abc')
  })
})

describe('roundToCurrency', () => {
  it('rounds to 0 dp for no-decimal currencies (IDR)', () => {
    expect(roundToCurrency('19493588.6896', 'IDR')).toBe('19493589')
    expect(roundToCurrency('19493588.4', 'IDR')).toBe('19493588')
  })
  it('rounds to 0 dp for other no-decimal currencies (JPY/KRW/VND)', () => {
    for (const ccy of ['JPY', 'KRW', 'VND']) {
      expect(roundToCurrency('1234.567', ccy)).toBe('1235')
    }
  })
  it('rounds to 2 dp for ordinary currencies (USD)', () => {
    expect(roundToCurrency('19493588.6896', 'USD')).toBe('19493588.69')
    expect(roundToCurrency('100', 'USD')).toBe('100.00')
  })
  it('returns the raw input when the amount is not a number', () => {
    expect(roundToCurrency('not-a-number', 'IDR')).toBe('not-a-number')
  })
})
