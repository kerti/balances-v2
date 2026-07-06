import { describe, it, expect } from "vitest";
import {
  STATUS_VALUES,
  statusLabel,
  statusOptions,
  isActiveStatus,
  type LifecycleGroup,
} from "@/lib/lifecycle";

const groups: LifecycleGroup[] = ["assets", "liabilities", "receivables", "investments"];

describe("STATUS_VALUES", () => {
  it("leads every group with the active value", () => {
    for (const group of groups) {
      expect(STATUS_VALUES[group][0]).toBe("active");
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
