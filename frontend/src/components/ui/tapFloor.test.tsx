import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Button } from "./button";
import { Input } from "./input";
import { Select } from "./select";

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

// `<select>` was the one control kind with no primitive to floor (#541): all 34
// callsites hand-rolled `h-9` = 36px, so every form re-opened the hole that
// #559 closed for buttons and text fields. The primitive exists so these
// assertions have somewhere to live.
describe("Select", () => {
  it("floors the control on phones", () => {
    render(
      <Select aria-label="Choice">
        <option value="a">A</option>
      </Select>,
    );
    expect(screen.getByRole("combobox")).toHaveClass("max-md:min-h-11");
  });

  it("lets a callsite opt out", () => {
    render(
      <Select aria-label="Choice" className="max-md:min-h-0">
        <option value="a">A</option>
      </Select>,
    );
    expect(screen.getByRole("combobox")).not.toHaveClass("max-md:min-h-11");
  });

  // iOS Safari zooms the viewport when a control under 16px takes focus, which
  // every one of the old `text-sm` selects did on every tap. The floor is only
  // half the fix; this pins the other half.
  it("keeps the 16px mobile font that stops iOS zooming on focus", () => {
    render(
      <Select aria-label="Choice">
        <option value="a">A</option>
      </Select>,
    );
    const el = screen.getByRole("combobox");
    expect(el).toHaveClass("text-base");
    expect(el).toHaveClass("md:text-sm");
  });
});
