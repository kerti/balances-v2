import { describe, expect, it } from "vitest";

import { resolveAppVersion, resolveDeployEnv } from "./appInfo";

describe("resolveAppVersion", () => {
  it("passes a release tag through unchanged", () => {
    expect(resolveAppVersion("v0.6.0-alpha.2")).toBe("v0.6.0-alpha.2");
  });

  it('falls back to "dev" when unset', () => {
    expect(resolveAppVersion(undefined)).toBe("dev");
  });

  it('falls back to "dev" on an empty string', () => {
    expect(resolveAppVersion("")).toBe("dev");
  });
});

describe("resolveDeployEnv", () => {
  it.each(["preview", "demo", "production", "self-hosted", "local"] as const)(
    "passes the known target %s through unchanged",
    (env) => {
      expect(resolveDeployEnv(env)).toBe(env);
    },
  );

  it('falls back to "local" when unset', () => {
    expect(resolveDeployEnv(undefined)).toBe("local");
  });

  it('falls back to "local" for an unrecognised value', () => {
    expect(resolveDeployEnv("staging")).toBe("local");
  });
});
