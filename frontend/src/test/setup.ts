import "@testing-library/jest-dom/vitest";
import { afterAll, afterEach, beforeAll } from "vitest";
import { cleanup } from "@testing-library/react";
import { server } from "./server";
import i18n, { i18nReady } from "@/i18n";

// jsdom omits a handful of layout/observer APIs the UI reaches for. Radix
// (dropdown menu, dialogs) calls the pointer-capture + scrollIntoView methods;
// `useIsMobile` subscribes to matchMedia. Stub them so a render doesn't throw.
// The mobile *value* is read from `window.innerWidth`, not matchMedia.matches,
// so a test flips layout by setting innerWidth, not by faking a media match.
if (!window.matchMedia) {
  window.matchMedia = ((query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addEventListener: () => {},
    removeEventListener: () => {},
    addListener: () => {},
    removeListener: () => {},
    dispatchEvent: () => false,
  })) as unknown as typeof window.matchMedia;
}
if (!window.ResizeObserver) {
  window.ResizeObserver = class {
    observe() {}
    unobserve() {}
    disconnect() {}
  };
}
Element.prototype.scrollIntoView = () => {};
Element.prototype.hasPointerCapture = () => false;
Element.prototype.setPointerCapture = () => {};
Element.prototype.releasePointerCapture = () => {};

beforeAll(async () => {
  // Bundled catalogs resolve synchronously, but await readiness so the first
  // `t()` returns real copy; pin the locale so assertions on rendered text are
  // deterministic regardless of the jsdom navigator language.
  await i18nReady;
  await i18n.changeLanguage("en-GB");
  server.listen({ onUnhandledRequest: "error" });
});

afterEach(() => {
  cleanup();
  server.resetHandlers();
});

afterAll(() => server.close());
