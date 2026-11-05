# Decimal precision for amounts, quantities, and FX rates

All numeric data is stored at uniform high precision; display formatting is currency-aware at render
time. The schema uses three precision shapes — one for monetary amounts, one for instrument
quantities, and one for FX rates — chosen for simplicity and future-proofing rather than
per-currency or per-subtype tuning.

## Decisions

| Concern | Column type | Notes |
|---|---|---|
| Monetary amount | `DECIMAL(20, 4)` | Every `(amount, currency)` value across snapshots, principal, face value, coupon amount, fee amount, income amount, etc. Headroom ~10¹⁶ in any currency. |
| Instrument quantity | `DECIMAL(20, 8)` | Stock / MutualFund / Gold quantities in snapshots and Buy/Sell transactions; future variable-quantity instruments. |
| FX rate | `DECIMAL(20, 8)` | Per-currency, per-month rates in the FX rate table (ADR-0002). |
| Rounding | half-up at display | Storage retains full precision; rounding occurs only at the display boundary. |

## Why uniform shapes

The natural alternative — per-currency precision (e.g., `INT` for IDR, `DECIMAL(20, 2)` for USD,
`DECIMAL(20, 3)` for KWD) or per-subtype quantity precision — is technically "correct" but
introduces currency- and subtype-aware code at every read/write site. For a personal-finance app
with manual data entry, the cost is real and the benefit is theoretical: no one enters fractional
rupiah, and storing 2 unused decimal places on every IDR amount costs nothing.

The 4-decimal amount shape gives FX-conversion intermediate computations room to operate without
rounding losses bleeding into stored values. The 8-decimal quantity and FX-rate shapes cover any
plausible future case (fractional shares, crypto-grade precision) without future migration.

## Display formatting (not a storage concern)

The frontend formats each stored value according to its currency's ISO 4217 convention:

- IDR → ~~no decimals~~ 2 decimals (`15.000.000,00`) — amended by [[adr-0056]]: ISO 4217 gives IDR 2 minor units
- USD → 2 decimals (`1,500.00`)
- BHD / KWD → 3 decimals
- Future crypto → up to 8 decimals

Rounding is half-up. The storage layer never rounds.

## Considered alternatives

- **`DECIMAL(20, 2)` for amounts.** Rejected — breaks on currencies with >2 decimals and loses
  FX-conversion intermediate precision.
- **Per-currency precision via separate column types.** Rejected — currency-aware code burden across
  every numeric column.
- **Integer minor units (cents, rupiah-as-integers).** Rejected — common in payments engineering for
  float-rounding safety, but requires currency-aware locale code at every read/write. Decimal types
  in SQL already avoid float-rounding issues.
- **Per-subtype quantity precision** (e.g., stock `DECIMAL(20, 4)`, mutual fund `DECIMAL(20, 6)`).
  Rejected — uniform `DECIMAL(20, 8)` is generous, simple, and uniform across the Go structs that
  read these tables.
- **Banker's rounding (half-to-even) at display.** Rejected — half-up matches user intuition; no
  regulatory or systematic-bias concern at personal-finance scale.

## Consequences

- Go structs mapping these columns use a decimal type (e.g., `shopspring/decimal`) rather than
  `float64` — chosen during the tech-stack round to match the schema's exact-decimal semantics.
- Display formatting per ISO 4217 is a frontend responsibility, not a database concern.
- Aggregations sum at full precision; rounding happens once at the boundary into the UI.
- Future currencies (crypto, weird inflation indexers) are absorbed without schema migration.
