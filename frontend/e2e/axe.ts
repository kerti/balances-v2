import { expect, type Page } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";

// Accessibility smoke assertion (#368 / CF-27). Runs axe-core against whatever
// the page currently shows and fails on any serious/critical WCAG 2.0/2.1 A+AA
// violation — the severities that actually block a non-technical household
// member (an unlabelled control a screen reader can't announce, unreadable
// contrast, a broken landmark/role), not the long tail of minor best-practice
// advice. Called at a settled point in a handful of @smoke specs so the surfaces
// every user meets first (sign-in, dashboard, a position screen) carry a
// standing a11y guard, rather than adding a separate authored flow.
//
// `context` names the screen under test so a failure reads which surface
// regressed without opening the trace.
export async function expectNoA11yViolations(page: Page, context: string): Promise<void> {
  const results = await new AxeBuilder({ page })
    .withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"])
    .analyze();

  const blocking = results.violations.filter(
    (v) => v.impact === "serious" || v.impact === "critical",
  );

  expect(blocking, formatViolations(context, blocking)).toEqual([]);
}

// Renders violations as a readable list — id, impact, help URL, and the first
// few offending selectors — so a CI failure is actionable from the log alone.
function formatViolations(
  context: string,
  violations: Awaited<ReturnType<AxeBuilder["analyze"]>>["violations"],
): string {
  if (violations.length === 0) return `no a11y violations on ${context}`;
  const lines = violations.map((v) => {
    const nodes = v.nodes
      .slice(0, 5)
      .map((n) => `      - ${n.target.join(" ")}`)
      .join("\n");
    return `  [${v.impact}] ${v.id}: ${v.help}\n    ${v.helpUrl}\n${nodes}`;
  });
  return `${violations.length} serious/critical a11y violation(s) on ${context}:\n${lines.join("\n")}`;
}
