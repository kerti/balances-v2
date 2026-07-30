import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Button } from "./button";
import { Input } from "./input";
import { Select } from "./select";
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSub,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from "./dropdown-menu";

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

  // The floor above is only a promise the platform has to keep, and for the
  // temporal types iOS Safari did not (#572): it draws a native control with its
  // own intrinsic width and height, ignoring `h-8` / `max-md:min-h-11` and
  // refusing to shrink into its grid column. Resetting the appearance is what
  // makes the rest of the primitive's box model — the floor included — apply at
  // all, so it belongs in this file rather than in a separate suite.
  it.each(["date", "month", "week", "time", "datetime-local"] as const)(
    "resets the native %s control so the app's own box model applies",
    (type) => {
      render(<Input aria-label="When" type={type} />);
      const el = screen.getByLabelText("When");
      expect(el).toHaveClass("appearance-none");
      expect(el).toHaveClass("max-md:min-h-11");
    },
  );

  // Scoped deliberately: `appearance-none` on a text field is inert, but on
  // `type="file"` it would strip the platform's "Choose file" button, which the
  // primitive's `file:*` classes style rather than replace.
  it.each(["text", "number", "email", "file"] as const)(
    "leaves the %s type's native appearance alone",
    (type) => {
      render(<Input aria-label="Field" type={type} />);
      expect(screen.getByLabelText("Field")).not.toHaveClass("appearance-none");
    },
  );
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

  // The floor above was not enough on its own either (#572): with the platform
  // appearance intact, iOS Safari draws a native control with its own intrinsic
  // height and ignores `h-8` / `max-md:min-h-11`, so every select in the app sat
  // shorter than the text field beside it on a real phone. #541 kept the native
  // arrow deliberately; the reset is what makes the height land, so the app now
  // draws its own chevron. Tapping still opens the platform's sheet either way.
  it("resets the native appearance so the floor can apply, and draws its own chevron", () => {
    render(
      <Select aria-label="Choice">
        <option value="a">A</option>
      </Select>,
    );
    const el = screen.getByRole("combobox");
    expect(el).toHaveClass("appearance-none");
    expect(el).toHaveClass("max-md:min-h-11");
    // A <select> can host neither child markup nor a pseudo-element, so the
    // replacement arrow is a background image; `pr-8` is the room it needs.
    expect(el).toHaveClass("bg-[image:var(--select-chevron)]");
    expect(el).toHaveClass("pr-8");
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

// The floor reached the ⋮ trigger in #510/#511/#535 but never what it opens: a
// menu item is `py-1` + `text-sm` = ~28px, so the very menu the floored trigger
// exists to reach handed a phone two under-floor targets (#572). Every
// interactive item kind carries the floor now; the non-interactive parts stay
// dense on purpose, since padding a label only makes the menu taller.
describe("DropdownMenu items", () => {
  function openMenu() {
    render(
      <DropdownMenu>
        <DropdownMenuTrigger>Open</DropdownMenuTrigger>
        <DropdownMenuContent>
          <DropdownMenuLabel>Group</DropdownMenuLabel>
          <DropdownMenuItem>Edit</DropdownMenuItem>
          <DropdownMenuCheckboxItem checked>Show closed</DropdownMenuCheckboxItem>
          <DropdownMenuRadioGroup value="a">
            <DropdownMenuRadioItem value="a">Ascending</DropdownMenuRadioItem>
          </DropdownMenuRadioGroup>
          <DropdownMenuSub>
            <DropdownMenuSubTrigger>More</DropdownMenuSubTrigger>
          </DropdownMenuSub>
        </DropdownMenuContent>
      </DropdownMenu>,
    );
    fireEvent.pointerDown(screen.getByText("Open"), { button: 0, ctrlKey: false });
  }

  it.each([
    ["menuitem", "Edit"],
    ["menuitemcheckbox", "Show closed"],
    ["menuitemradio", "Ascending"],
  ] as const)("floors the %s on phones", (role, name) => {
    openMenu();
    expect(screen.getByRole(role, { name })).toHaveClass("max-md:min-h-11");
  });

  it("floors the submenu trigger on phones", () => {
    openMenu();
    expect(screen.getByText("More")).toHaveClass("max-md:min-h-11");
  });

  it("leaves the non-interactive label dense", () => {
    openMenu();
    expect(screen.getByText("Group")).not.toHaveClass("max-md:min-h-11");
  });
});
