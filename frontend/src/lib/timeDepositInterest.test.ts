import { describe, expect, it } from "vitest";
import { timeDepositInterest } from "@/lib/timeDepositInterest";

describe("timeDepositInterest", () => {
  it("is Value minus the principal in force", () => {
    expect(timeDepositInterest("101500000", "100000000")).toEqual({
      valid: true,
      amount: "1500000",
      below: false,
    });
  });

  it("is zero, not below, when Value equals the principal", () => {
    expect(timeDepositInterest("100000000", "100000000")).toEqual({
      valid: true,
      amount: "0",
      below: false,
    });
  });

  it("goes negative and flags below when Value is under the principal", () => {
    expect(timeDepositInterest("99000000", "100000000")).toEqual({
      valid: true,
      amount: "-1000000",
      below: true,
    });
  });

  it("keeps cents exact despite float noise", () => {
    expect(timeDepositInterest("1234.56", "1000.5").amount).toBe("234.06");
  });

  it("is not valid until both figures are numbers", () => {
    expect(timeDepositInterest("", "100").valid).toBe(false);
    expect(timeDepositInterest("abc", "100").valid).toBe(false);
    expect(timeDepositInterest("100", null).valid).toBe(false);
    expect(timeDepositInterest("100", undefined).valid).toBe(false);
    expect(timeDepositInterest("100", "").valid).toBe(false);
  });
});
