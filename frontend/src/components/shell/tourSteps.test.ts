import { describe, expect, it } from "vitest";
import { toDriveSteps } from "@/components/shell/tourSteps";

const steps = [
  { element: "[data-testid=a]", title: "A", description: "a" },
  { element: "[data-testid=b]", title: "B", description: "b" },
  { title: "C", description: "c" },
];

describe("toDriveSteps", () => {
  it("hides Back on the first step only", () => {
    const out = toDriveSteps(steps);
    expect(out[0].popover?.showButtons).toEqual(["next", "close"]);
    expect(out[1].popover?.showButtons).toBeUndefined();
    expect(out[2].popover?.showButtons).toBeUndefined();
  });

  it("treats the first of the already-pruned steps as first", () => {
    const out = toDriveSteps(steps.slice(1));
    expect(out[0].popover?.title).toBe("B");
    expect(out[0].popover?.showButtons).toEqual(["next", "close"]);
    expect(out[1].popover?.showButtons).toBeUndefined();
  });
});
