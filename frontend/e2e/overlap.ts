import { expect, type Locator, type Page } from "@playwright/test";

// A measured guard against interactive controls drawn on top of each other
// (#865: the month picker ran 48px over the statement-date field at 1280px and
// 85px at 390px, and nobody saw it until it was measured). Collects the visible
// controls inside `scope` (default: the whole page), takes each one's
// on-screen box, and fails when two boxes intersect by more than 1px on both
// axes.
//
// A control is left out when it is zero-size or visually hidden (sr-only,
// opacity 0, visibility hidden, display none), sits under an aria-hidden/inert
// ancestor, or its visible part (clipped by every scrolling or overflow-hidden
// ancestor) is empty or entirely outside the viewport horizontally. A pair is
// left out when one element contains the other in the DOM (a radio inside its
// label-button, a checkbox inside a row button).
const CONTROLS = [
  "button",
  "a[href]",
  'input:not([type="hidden"])',
  "select",
  "textarea",
  '[role="combobox"]',
  '[role="switch"]',
  '[role="checkbox"]',
  '[role="radio"]',
  '[role="tab"]',
].join(",");

type Overlap = { a: string; b: string; px: string };

export async function expectNoControlOverlap(page: Page, scope?: Locator, label = "") {
  await settle(page);
  const handle = scope
    ? await scope.elementHandle()
    : await page.evaluateHandle(() => document.body);
  if (!handle) throw new Error("expectNoControlOverlap: scope not found");

  const overlaps = await page.evaluate(
    ({ root, selector }) => {
      type Box = { left: number; top: number; right: number; bottom: number };
      const viewportWidth = document.documentElement.clientWidth;

      const hidden = (el: Element) => {
        for (let n: Element | null = el; n; n = n.parentElement) {
          const cs = getComputedStyle(n);
          if (cs.display === "none" || cs.visibility === "hidden" || Number(cs.opacity) === 0) {
            return true;
          }
          if (n.getAttribute("aria-hidden") === "true" || n.hasAttribute("inert")) return true;
        }
        return false;
      };

      // The part of the element a person can see: its box intersected with the
      // box of every ancestor that clips (overflow other than visible).
      const visibleBox = (el: Element): Box | null => {
        const r = el.getBoundingClientRect();
        if (r.width <= 1 && r.height <= 1) return null;
        if (r.width === 0 || r.height === 0) return null;
        const box: Box = { left: r.left, top: r.top, right: r.right, bottom: r.bottom };
        for (let n = el.parentElement; n && n !== document.documentElement; n = n.parentElement) {
          if (n === document.body) continue;
          const cs = getComputedStyle(n);
          if (cs.overflowX === "visible" && cs.overflowY === "visible") continue;
          const c = n.getBoundingClientRect();
          box.left = Math.max(box.left, c.left);
          box.top = Math.max(box.top, c.top);
          box.right = Math.min(box.right, c.right);
          box.bottom = Math.min(box.bottom, c.bottom);
          if (box.right <= box.left || box.bottom <= box.top) return null;
        }
        if (box.right <= 0 || box.left >= viewportWidth) return null;
        return box;
      };

      const describe = (el: Element) => {
        const labelled = el.getAttribute("aria-labelledby");
        const name =
          el.getAttribute("aria-label") ||
          (labelled && document.getElementById(labelled)?.textContent) ||
          (el as HTMLInputElement).labels?.[0]?.textContent ||
          el.textContent ||
          el.getAttribute("placeholder") ||
          el.getAttribute("title") ||
          "";
        const role = el.getAttribute("role") || el.tagName.toLowerCase();
        const testid = el.getAttribute("data-testid");
        return `${role} "${name.trim().replace(/\s+/g, " ").slice(0, 40)}"${testid ? ` [${testid}]` : ""}`;
      };

      const items: { el: Element; box: Box }[] = [];
      for (const el of (root as Element).querySelectorAll(selector)) {
        if (hidden(el)) continue;
        const box = visibleBox(el);
        if (box) items.push({ el, box });
      }

      const found: { a: string; b: string; px: string }[] = [];
      for (let i = 0; i < items.length; i++) {
        for (let j = i + 1; j < items.length; j++) {
          const p = items[i];
          const q = items[j];
          if (p.el.contains(q.el) || q.el.contains(p.el)) continue;
          const w = Math.min(p.box.right, q.box.right) - Math.max(p.box.left, q.box.left);
          const h = Math.min(p.box.bottom, q.box.bottom) - Math.max(p.box.top, q.box.top);
          if (w > 1 && h > 1) {
            found.push({
              a: describe(p.el),
              b: describe(q.el),
              px: `${Math.round(w)}x${Math.round(h)}px`,
            });
          }
        }
      }
      return found;
    },
    { root: handle, selector: CONTROLS },
  );

  const lines = overlaps.map((o: Overlap) => `  ${o.a}  overlaps  ${o.b}  by ${o.px}`);
  expect.soft(lines, `${label ? `${label}: ` : ""}interactive controls overlap`).toEqual([]);
}

// Dialogs and drawers animate in; measure only once finite animations and web
// fonts are done and layout has had two frames to settle.
async function settle(page: Page) {
  await page.evaluate(async () => {
    await document.fonts.ready;
    await Promise.all(
      document
        .getAnimations()
        .filter((a) => a.effect?.getComputedTiming().iterations !== Infinity)
        .map((a) => a.finished.catch(() => undefined)),
    );
    await new Promise<void>((resolve) =>
      requestAnimationFrame(() => requestAnimationFrame(() => resolve())),
    );
  });
}
