import type { DriveStep } from "driver.js";

type TourStepInput = { element?: string; title: string; description: string };

// The first step actually shown has nothing to go back to, so Back is hidden
// there; "first" is judged after steps with an absent anchor were pruned.
export function toDriveSteps(present: TourStepInput[]): DriveStep[] {
  return present.map((s, i) => ({
    element: s.element,
    popover: {
      title: s.title,
      description: s.description,
      ...(i === 0 ? { showButtons: ["next", "close"] as ("next" | "close")[] } : {}),
    },
  }));
}
