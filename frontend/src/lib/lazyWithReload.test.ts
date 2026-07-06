import { describe, expect, it, vi } from "vitest";
import { importWithReloadGuard, isChunkLoadError, type ReloadEnv } from "@/lib/lazyWithReload";

describe("isChunkLoadError", () => {
  it("matches the engine messages for a missing dynamic chunk", () => {
    for (const msg of [
      "Failed to fetch dynamically imported module: https://x/assets/Chart-abc.js",
      "error loading dynamically imported module",
      "Importing a module script failed.",
    ]) {
      expect(isChunkLoadError(new Error(msg))).toBe(true);
    }
  });

  it("does not match unrelated errors", () => {
    expect(isChunkLoadError(new Error("boom"))).toBe(false);
    expect(isChunkLoadError("plain string")).toBe(false);
    expect(isChunkLoadError(undefined)).toBe(false);
  });
});

describe("importWithReloadGuard", () => {
  // In-memory stand-in for the browser env so the guard logic tests in `node`.
  function fakeEnv(initial = false) {
    let guard = initial;
    const env: ReloadEnv = {
      guardSet: () => guard,
      setGuard: () => {
        guard = true;
      },
      clearGuard: () => {
        guard = false;
      },
      reload: vi.fn(),
    };
    return env;
  }

  const chunkErr = () => Promise.reject(new Error("Failed to fetch dynamically imported module"));

  it("passes a successful import through and clears any stale guard", async () => {
    const env = fakeEnv(true);
    const mod = { default: "ok" };
    await expect(importWithReloadGuard(() => Promise.resolve(mod), env)).resolves.toBe(mod);
    expect(env.guardSet()).toBe(false);
    expect(env.reload).not.toHaveBeenCalled();
  });

  it("reloads once and sets the guard on a first chunk failure", async () => {
    const env = fakeEnv(false);
    // Holds the Suspense fallback (never settles); race it against a tick.
    const pending = importWithReloadGuard(chunkErr, env);
    const settled = await Promise.race([pending.then(() => "settled"), Promise.resolve("pending")]);
    expect(settled).toBe("pending");
    expect(env.reload).toHaveBeenCalledOnce();
    expect(env.guardSet()).toBe(true);
  });

  it("rethrows a chunk failure once the guard is already set (no loop)", async () => {
    const env = fakeEnv(true);
    await expect(importWithReloadGuard(chunkErr, env)).rejects.toThrow(
      /dynamically imported module/,
    );
    expect(env.reload).not.toHaveBeenCalled();
  });

  it("rethrows a non-chunk error without reloading", async () => {
    const env = fakeEnv(false);
    await expect(
      importWithReloadGuard(() => Promise.reject(new Error("boom")), env),
    ).rejects.toThrow("boom");
    expect(env.reload).not.toHaveBeenCalled();
  });
});
