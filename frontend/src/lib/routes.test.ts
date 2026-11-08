import { describe, it, expect } from "vitest";

import { routes, positionDetail, positionEntry } from "@/lib/routes";

// The route builders are the link-safety convention (ADR-0025): these assert
// each id-builder nests its detail under the matching list path, so a typo in
// a builder is caught here rather than as a runtime 404.

describe("routes builders", () => {
  it("nests each subtype detail under its list path", () => {
    expect(routes.bankAccount("a1")).toBe(`${routes.bankAccounts}/a1`);
    expect(routes.property("p1")).toBe(`${routes.properties}/p1`);
    expect(routes.vehicle("v1")).toBe(`${routes.vehicles}/v1`);
    expect(routes.receivable("r1")).toBe(`${routes.receivables}/r1`);
    expect(routes.stock("s1")).toBe(`${routes.stocks}/s1`);
    expect(routes.mutualFund("m1")).toBe(`${routes.mutualFunds}/m1`);
    expect(routes.bond("b1")).toBe(`${routes.bonds}/b1`);
    expect(routes.timeDeposit("t1")).toBe(`${routes.timeDeposits}/t1`);
    expect(routes.goldItem("g1")).toBe(`${routes.gold}/g1`);
  });

  it("builds the liability detail under its subtype segment", () => {
    expect(routes.liability("personal", "l1")).toBe("/liabilities/personal/l1");
    expect(routes.liability("institutional", "l2")).toBe("/liabilities/institutional/l2");
  });

  it("exposes the static group/list paths", () => {
    expect(routes.dashboard).toBe("/");
    expect(routes.assets).toBe("/assets");
    expect(routes.investments).toBe("/investments");
    expect(routes.income).toBe("/income");
    expect(routes.settings).toBe("/settings");
  });
});

// positionDetail backs the stale-position drill-down (#50): the report's
// (group, subtype) wire values must resolve to the same detail paths the
// builders produce, and unknown pairs must degrade to null (plain label).
describe("positionDetail", () => {
  it("resolves every (group, subtype) the report can emit", () => {
    expect(positionDetail("asset", "bank_account", "a1")).toBe(routes.bankAccount("a1"));
    expect(positionDetail("asset", "property", "p1")).toBe(routes.property("p1"));
    expect(positionDetail("asset", "vehicle", "v1")).toBe(routes.vehicle("v1"));
    expect(positionDetail("liability", "personal", "l1")).toBe(routes.liability("personal", "l1"));
    expect(positionDetail("liability", "institutional", "l2")).toBe(
      routes.liability("institutional", "l2"),
    );
    expect(positionDetail("receivable", "", "r1")).toBe(routes.receivable("r1"));
    expect(positionDetail("investment", "stock", "s1")).toBe(routes.stock("s1"));
    expect(positionDetail("investment", "mutual_fund", "m1")).toBe(routes.mutualFund("m1"));
    expect(positionDetail("investment", "bond", "b1")).toBe(routes.bond("b1"));
    expect(positionDetail("investment", "time_deposit", "t1")).toBe(routes.timeDeposit("t1"));
    expect(positionDetail("investment", "gold", "g1")).toBe(routes.goldItem("g1"));
  });

  it("returns null for unknown group/subtype pairs", () => {
    expect(positionDetail("asset", "mystery", "x")).toBeNull();
    expect(positionDetail("investment", "mystery", "x")).toBeNull();
    expect(positionDetail("liability", "mystery", "x")).toBeNull();
    expect(positionDetail("nonsense", "whatever", "x")).toBeNull();
  });
});

describe("positionEntry", () => {
  it("maps each (group, subtype) to its bulk-entry route", () => {
    expect(positionEntry("asset", "property")).toBe(routes.assetsEnter);
    expect(positionEntry("liability", "personal")).toBe(routes.liabilitiesEnter);
    expect(positionEntry("receivable", "")).toBe(routes.receivablesEnter);
    for (const st of ["stock", "mutual_fund", "gold"]) {
      expect(positionEntry("investment", st)).toBe(routes.investmentsEnterPrices);
    }
    for (const st of ["bond", "time_deposit"]) {
      expect(positionEntry("investment", st)).toBe(routes.investmentsEnterAccrued);
    }
  });

  it("returns null for an unknown pair", () => {
    expect(positionEntry("investment", "crypto")).toBeNull();
    expect(positionEntry("mystery", "x")).toBeNull();
  });
});
