import { describe, it, expect } from "vitest";

import { ApiError } from "@/api/client";
import { shouldRetryQuery } from "@/queryClient";

describe("shouldRetryQuery", () => {
  it("never retries a 4xx — identical bytes will fail identically", () => {
    expect(shouldRetryQuery(0, new ApiError(404, "not found"))).toBe(false);
    expect(shouldRetryQuery(0, new ApiError(400, "bad request"))).toBe(false);
    expect(shouldRetryQuery(0, new ApiError(401, "unauthorized"))).toBe(false);
  });

  it("keeps the default 3-attempt budget for a 5xx", () => {
    expect(shouldRetryQuery(0, new ApiError(500, "server error"))).toBe(true);
    expect(shouldRetryQuery(2, new ApiError(500, "server error"))).toBe(true);
    expect(shouldRetryQuery(3, new ApiError(500, "server error"))).toBe(false);
  });

  it("keeps the default budget for a non-ApiError (network failure, timeout)", () => {
    expect(shouldRetryQuery(0, new TypeError("Failed to fetch"))).toBe(true);
    expect(shouldRetryQuery(3, new TypeError("Failed to fetch"))).toBe(false);
  });
});
