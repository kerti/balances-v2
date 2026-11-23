import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { Label } from "@/components/ui/label";

// #953 S12: one marker for every required field. The "*" is drawn by CSS, so the
// label's text stays exactly what the household reads and tests query.
describe("Label required marker", () => {
  it("flags a required label without changing its text", () => {
    render(
      <>
        <Label required htmlFor="a">
          Bank name
        </Label>
        <input id="a" required />
        <Label htmlFor="b">Note</Label>
        <input id="b" />
      </>,
    );
    expect(screen.getByText("Bank name")).toHaveAttribute("data-required", "true");
    expect(screen.getByLabelText("Bank name")).toBeRequired();
    expect(screen.getByText("Note")).not.toHaveAttribute("data-required");
  });
});
