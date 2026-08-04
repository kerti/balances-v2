import { describe, it, expect } from "vitest";
import {
  STATUS_VALUES,
  statusLabel,
  statusOptions,
  settlementKind,
  isActiveStatus,
  type InvestmentSubtype,
  type LifecycleGroup,
} from "@/lib/lifecycle";

const groups: LifecycleGroup[] = ["assets", "liabilities", "receivables", "investments"];

describe("STATUS_VALUES", () => {
  it("leads every group with the active value", () => {
    for (const group of groups) {
      expect(STATUS_VALUES[group][0]).toBe("active");
    }
  });

  // covers: INV-LIFECYCLE-09
  it("closes every group with untracked — the one terminal status all four share", () => {
    for (const group of groups) {
      const values = STATUS_VALUES[group];
      expect(values[values.length - 1]).toBe("untracked");
    }
  });
});

describe("statusOptions", () => {
  it("preserves the per-group order and pairs each value with a label", () => {
    for (const group of groups) {
      const opts = statusOptions(group);
      expect(opts.map((o) => o.value)).toEqual(STATUS_VALUES[group]);
      for (const o of opts) {
        // i18n is not initialised in unit tests; the lookup returns the
        // defaultValue (the raw status) — still a non-empty string we can
        // surface in a dropdown. Catalog correctness is asserted in
        // i18n/catalogs.test.ts.
        expect(typeof o.label).toBe("string");
        expect(o.label.length).toBeGreaterThan(0);
      }
    }
  });
});

describe("statusOptions for an investment subtype", () => {
  // covers: INV-LIFECYCLE-08
  it("offers only the terminal statuses the subtype's transaction matrix can settle", () => {
    const values = (subtype: InvestmentSubtype) =>
      statusOptions("investments", subtype).map((o) => o.value);

    // Sell-only subtypes: a Stock cannot mature. `untracked` survives the
    // narrowing for every subtype — it settles nothing, so the matrix has no
    // say over it (ADR-0053 §5).
    expect(values("stock")).toEqual(["active", "sold", "untracked"]);
    expect(values("mutual_fund")).toEqual(["active", "sold", "untracked"]);
    expect(values("gold")).toEqual(["active", "sold", "untracked"]);
    // A Bond takes either.
    expect(values("bond")).toEqual(["active", "sold", "matured", "untracked"]);
    // A TimeDeposit accepts only Maturity, so it cannot be "sold".
    expect(values("time_deposit")).toEqual(["active", "matured", "untracked"]);
  });

  // covers: INV-LIFECYCLE-08
  it("keeps a current status the narrowing would otherwise drop", () => {
    // A deposit already recorded as 'sold' — by import, restore, or before the
    // narrowing existed. Dropping it would blank the dropdown's own value.
    expect(statusOptions("investments", "time_deposit", "sold").map((o) => o.value)).toEqual([
      "active",
      "matured",
      "untracked",
      "sold",
    ]);
    // A status already on the list is never duplicated.
    expect(statusOptions("investments", "stock", "sold").map((o) => o.value)).toEqual([
      "active",
      "sold",
      "untracked",
    ]);
    // covers: INV-LIFECYCLE-09
    // ...including untracked itself: re-opening the dialog on a position that
    // already left the books must not blank its own value.
    expect(statusOptions("investments", "time_deposit", "untracked").map((o) => o.value)).toEqual([
      "active",
      "matured",
      "untracked",
    ]);
  });

  it("leaves the other three groups unnarrowed", () => {
    for (const group of ["assets", "liabilities", "receivables"] as LifecycleGroup[]) {
      expect(statusOptions(group).map((o) => o.value)).toEqual(STATUS_VALUES[group]);
    }
  });
});

describe("settlementKind", () => {
  // covers: INV-LIFECYCLE-08
  it("maps each settleable pair to the transaction that expresses it", () => {
    expect(settlementKind("stock", "sold")).toBe("sell");
    expect(settlementKind("gold", "sold")).toBe("sell");
    expect(settlementKind("bond", "sold")).toBe("sell");
    expect(settlementKind("bond", "matured")).toBe("maturity");
    expect(settlementKind("time_deposit", "matured")).toBe("maturity");
  });

  // covers: INV-LIFECYCLE-08
  it("is null for a pair no transaction can express", () => {
    expect(settlementKind("stock", "matured")).toBeNull();
    expect(settlementKind("time_deposit", "sold")).toBeNull();
    expect(settlementKind("stock", "active")).toBeNull();
  });

  // covers: INV-LIFECYCLE-09
  it("is null for untracked on every subtype — nothing was sold", () => {
    const subtypes: InvestmentSubtype[] = ["stock", "mutual_fund", "gold", "bond", "time_deposit"];
    for (const subtype of subtypes) {
      expect(settlementKind(subtype, "untracked")).toBeNull();
    }
  });
});

describe("statusLabel", () => {
  it("falls back to the raw value when no translation is loaded", () => {
    // Without an initialised i18n the helper returns the defaultValue, which
    // is the status key itself — handy for tests and a safe runtime fallback.
    expect(statusLabel("receivables", "sold")).toBe("sold");
    expect(statusLabel("assets", "mystery")).toBe("mystery");
  });
});

describe("isActiveStatus", () => {
  it('is true only for "active"', () => {
    expect(isActiveStatus("active")).toBe(true);
    expect(isActiveStatus("closed")).toBe(false);
    expect(isActiveStatus("matured")).toBe(false);
    expect(isActiveStatus("")).toBe(false);
  });
});
