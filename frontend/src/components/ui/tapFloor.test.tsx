import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Button } from "./button";
import { Input } from "./input";

// The mobile tap floor now lives in the shared primitives rather than at each
// callsite (ADR-0050 / INV-PRESENTATION-08). Every sweep before this one
// (#507–#542) added `min-h-11 md:min-h-0` button by button, so each new screen
// re-opened the hole; these assertions pin the floor to the primitive so a
// regression shows up here instead of on a phone.
//
// jsdom has no layout, so the assertion is on the class contract, not measured
// pixels — the pixel truth is asserted at 390px in the @smoke specs
// (settings-mobile.spec.ts and friends).
//
// covers: INV-PRESENTATION-08
describe("Button", () => {
  it.each(["default", "sm", "lg"] as const)("floors the %s text size on phones", (size) => {
    render(<Button size={size}>Press</Button>);
    expect(screen.getByRole("button")).toHaveClass("max-md:min-h-11");
  });

  // Icon buttons are square: a bare `min-h-11` would render them 44×32. They
  // are floored at their callsites with `size-11` instead, so the variant must
  // stay clean — see the comment in button.tsx.
  it.each(["icon", "icon-sm", "icon-lg"] as const)("leaves the %s size square", (size) => {
    render(<Button size={size} aria-label="Act" />);
    expect(screen.getByRole("button")).not.toHaveClass("max-md:min-h-11");
  });

  // `xs` is a deliberate dense affordance, not an oversight.
  it("leaves the xs size dense", () => {
    render(<Button size="xs">Press</Button>);
    expect(screen.getByRole("button")).not.toHaveClass("max-md:min-h-11");
  });

  // The floor is a default, not a cage — tailwind-merge lets a callsite that
  // genuinely needs a denser control take the class back.
  it("lets a callsite opt out", () => {
    render(<Button className="max-md:min-h-0">Press</Button>);
    expect(screen.getByRole("button")).not.toHaveClass("max-md:min-h-11");
  });
});

describe("Input", () => {
  it("floors the field on phones", () => {
    render(<Input aria-label="Field" />);
    expect(screen.getByRole("textbox")).toHaveClass("max-md:min-h-11");
  });

  it("lets a callsite opt out", () => {
    render(<Input aria-label="Field" className="max-md:min-h-0" />);
    expect(screen.getByRole("textbox")).not.toHaveClass("max-md:min-h-11");
  });
});
