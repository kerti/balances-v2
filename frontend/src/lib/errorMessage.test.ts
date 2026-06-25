import { describe, it, expect } from "vitest";

import i18n from "@/i18n";
import { ApiError } from "@/api/client";
import { errorMessage } from "@/lib/errorMessage";

// Assertions read the live catalog via i18n.t(...) rather than hardcoding
// English copy, so a copy edit in errors.json doesn't break these tests — the
// behaviour under test is the envelope→key resolution, not the wording.

describe("errorMessage", () => {
  it("resolves an envelope code to its catalog string", () => {
    const err = new ApiError(404, "not found", { code: "NOT_FOUND" });
    expect(errorMessage(err)).toBe(i18n.t("errors:code.NOT_FOUND"));
  });

  it("interpolates envelope args into the template", () => {
    const err = new ApiError(400, "bad id", {
      code: "INVALID_ID",
      args: { field: "asset_id" },
    });
    expect(errorMessage(err)).toBe(
      i18n.t("errors:code.INVALID_ID", { field: "asset_id" }),
    );
  });

  it("resolves a known VALIDATION rule sub-key into the outer template", () => {
    const err = new ApiError(400, "validation", {
      code: "VALIDATION",
      args: { field: "email", rule: "email" },
    });
    const rule = i18n.t("errors:code.VALIDATION_RULE.email");
    expect(errorMessage(err)).toBe(
      i18n.t("errors:code.VALIDATION", { field: "email", rule }),
    );
  });

  it("falls back to the raw rule token for an unknown VALIDATION rule", () => {
    const err = new ApiError(400, "validation", {
      code: "VALIDATION",
      args: { field: "amount", rule: "mystery_rule" },
    });
    expect(errorMessage(err)).toBe(
      i18n.t("errors:code.VALIDATION", {
        field: "amount",
        rule: "mystery_rule",
      }),
    );
  });

  it("maps an unknown envelope code to the generic UNKNOWN copy", () => {
    const err = new ApiError(500, "weird", { code: "TOTALLY_FAKE_CODE" });
    expect(errorMessage(err)).toBe(i18n.t("errors:code.UNKNOWN"));
  });

  it("returns UNKNOWN for an ApiError whose body is a raw string", () => {
    const err = new ApiError(500, "oops", "plain text body");
    expect(errorMessage(err)).toBe(i18n.t("errors:code.UNKNOWN"));
  });

  it("returns UNKNOWN for an ApiError with no body", () => {
    const err = new ApiError(500, "oops");
    expect(errorMessage(err)).toBe(i18n.t("errors:code.UNKNOWN"));
  });

  it("surfaces a native Error message verbatim", () => {
    expect(errorMessage(new Error("network down"))).toBe("network down");
  });

  it("uses the fallback arg for a non-Error throw when given one", () => {
    expect(errorMessage("a thrown string", "custom fallback")).toBe(
      "custom fallback",
    );
  });

  it("falls back to UNKNOWN for a non-Error throw with no fallback", () => {
    expect(errorMessage({ weird: true })).toBe(i18n.t("errors:code.UNKNOWN"));
  });
});
