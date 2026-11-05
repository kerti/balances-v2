import { useState } from "react";
import { act, fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import i18n from "@/i18n";
import { NumericInput } from "@/components/common/NumericInput";

// Ported from kerti/uruni's AmountInput tests (ADR-0056), widened from whole
// rupiah to a decimal model: digits plus at most one decimal point.

function Controlled({
  onChange,
  decimals = 2,
  maxIntegerDigits,
  allowNegative,
  initial = "",
}: {
  onChange: (v: string) => void;
  decimals?: number;
  maxIntegerDigits?: number;
  allowNegative?: boolean;
  initial?: string;
}) {
  const [value, setValue] = useState(initial);
  return (
    <>
      <label htmlFor="amt">Amount</label>
      <NumericInput
        id="amt"
        decimals={decimals}
        maxIntegerDigits={maxIntegerDigits}
        allowNegative={allowNegative}
        value={value}
        onChange={(v) => {
          setValue(v);
          onChange(v);
        }}
      />
    </>
  );
}

function setup(props: Partial<Parameters<typeof Controlled>[0]> = {}) {
  const onChange = vi.fn();
  render(<Controlled onChange={onChange} {...props} />);
  const input = screen.getByLabelText("Amount") as HTMLInputElement;
  return { onChange, input };
}

// The focus handler moves the caret a frame later; let that frame pass before a
// test places the caret itself, or it lands late and moves the caret to the end.
async function flushFrame() {
  await act(() => new Promise<void>((resolve) => requestAnimationFrame(() => resolve())));
}

afterEach(async () => {
  await i18n.changeLanguage("en-GB");
});

// covers: INV-PRESENTATION-13
describe("NumericInput", () => {
  it("groups as you type and emits the canonical decimal string (en-GB)", async () => {
    const { onChange, input } = setup();
    await userEvent.type(input, "48500000");
    expect(input.value).toBe("48,500,000");
    await userEvent.type(input, ",50");
    expect(input.value).toBe("48,500,000.50");
    expect(onChange).toHaveBeenLastCalledWith("48500000.50");
  });

  it("groups in the id-ID marks, and a typed . is the decimal point there too", async () => {
    await i18n.changeLanguage("id-ID");
    const { onChange, input } = setup();
    await userEvent.type(input, "48500000");
    expect(input.value).toBe("48.500.000");
    await userEvent.type(input, ".50");
    expect(input.value).toBe("48.500.000,50");
    expect(onChange).toHaveBeenLastCalledWith("48500000.50");
  });

  it("drops anything but digits and a separator", async () => {
    const { onChange, input } = setup();
    await userEvent.type(input, "1a5b0-0");
    expect(input.value).toBe("1,500");
    expect(onChange).toHaveBeenLastCalledWith("1500");
  });

  it("ignores a typed separator on a field with no decimal places", async () => {
    const { onChange, input } = setup({ decimals: 0 });
    await userEvent.type(input, "48,500,000");
    expect(input.value).toBe("48,500,000");
    expect(onChange).toHaveBeenLastCalledWith("48500000");
  });

  it("refuses a second decimal point and a decimal past the limit", async () => {
    const { onChange, input } = setup();
    await userEvent.type(input, "1.234.5");
    expect(input.value).toBe("1.23");
    expect(onChange).toHaveBeenLastCalledWith("1.23");
  });

  it("refuses an integer digit past the limit", async () => {
    const { input } = setup({ maxIntegerDigits: 4 });
    await userEvent.type(input, "123456");
    expect(input.value).toBe("1,234");
  });

  it("drops a leading zero but keeps 0 before a decimal point", async () => {
    const { onChange, input } = setup();
    await userEvent.type(input, "05");
    expect(input.value).toBe("5");
    await userEvent.clear(input);
    await userEvent.type(input, ".5");
    expect(input.value).toBe("0.5");
    expect(onChange).toHaveBeenLastCalledWith("0.5");
  });

  it("emits without a trailing decimal point while one is half-typed", async () => {
    const { onChange, input } = setup();
    await userEvent.type(input, "12.");
    expect(input.value).toBe("12.");
    expect(onChange).toHaveBeenLastCalledWith("12");
  });

  it("keeps the caret after the same digit when a separator appears or goes", async () => {
    const { onChange, input } = setup();
    await userEvent.type(input, "45000");
    await flushFrame();
    expect(input.value).toBe("45,000");

    // A digit typed after the "4": 415,000 with the caret after the "1".
    await userEvent.type(input, "1", { initialSelectionStart: 1, initialSelectionEnd: 1 });
    expect(input.value).toBe("415,000");
    expect(input.selectionStart).toBe(2);

    // Backspace there takes the "1" back, caret after the "4".
    await userEvent.type(input, "{Backspace}", {
      initialSelectionStart: 2,
      initialSelectionEnd: 2,
    });
    expect(input.value).toBe("45,000");
    expect(input.selectionStart).toBe(1);
    expect(onChange).toHaveBeenLastCalledWith("45000");
  });

  it("takes the digit before a group separator on backspace", async () => {
    const { onChange, input } = setup();
    await userEvent.type(input, "45000");
    await flushFrame();
    // Caret just after the comma in 45,|000.
    await userEvent.type(input, "{Backspace}", {
      initialSelectionStart: 3,
      initialSelectionEnd: 3,
    });
    expect(input.value).toBe("4,000");
    expect(onChange).toHaveBeenLastCalledWith("4000");
  });

  it("takes the digit after a group separator on forward delete", async () => {
    const { onChange, input } = setup();
    await userEvent.type(input, "45000");
    await flushFrame();
    // Caret just before the comma in 45|,000.
    await userEvent.type(input, "{Delete}", {
      initialSelectionStart: 2,
      initialSelectionEnd: 2,
    });
    expect(input.value).toBe("4,500");
    expect(onChange).toHaveBeenLastCalledWith("4500");
  });

  it("replaces a selected value with what is typed", async () => {
    const { onChange, input } = setup({ initial: "50000" });
    expect(input.value).toBe("50,000");
    await userEvent.type(input, "23456", {
      initialSelectionStart: 0,
      initialSelectionEnd: input.value.length,
    });
    expect(input.value).toBe("23,456");
    expect(onChange).toHaveBeenLastCalledWith("23456");
  });

  it("prefills a stored value formatted, keeping decimals past the field's default", async () => {
    const { onChange, input } = setup({ initial: "1234.5678", decimals: 2 });
    expect(input.value).toBe("1,234.5678");
    await userEvent.type(input, "{Backspace}");
    expect(onChange).toHaveBeenLastCalledWith("1234.567");
  });

  it("reads a paste in the locale's grouped form", async () => {
    const { onChange, input } = setup();
    await userEvent.click(input);
    await userEvent.paste("Rp 48,500,000.50");
    expect(input.value).toBe("48,500,000.50");
    expect(onChange).toHaveBeenLastCalledWith("48500000.50");
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("refuses an unreadable paste with an inline message, keeping the value", async () => {
    const { onChange, input } = setup();
    await userEvent.type(input, "500");
    await userEvent.paste("1,5");
    expect(input.value).toBe("500");
    expect(onChange).toHaveBeenLastCalledWith("500");
    expect(screen.getByRole("alert")).toHaveTextContent(/number/i);
    expect(input).toHaveAttribute("aria-invalid", "true");

    // The message goes once the field takes an edit.
    await userEvent.type(input, "0");
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("refuses a paste over the field's limit whole, rather than cutting it", async () => {
    const { onChange, input } = setup({ maxIntegerDigits: 6 });
    await userEvent.type(input, "500");
    await userEvent.paste("1,000,000,000");
    expect(input.value).toBe("500");
    expect(onChange).toHaveBeenLastCalledWith("500");
    expect(screen.getByRole("alert")).toBeInTheDocument();
  });

  it("refuses a paste with more decimals than the field takes", async () => {
    const { input } = setup({ decimals: 2 });
    await userEvent.click(input);
    await userEvent.paste("1.234");
    expect(input.value).toBe("");
    expect(screen.getByRole("alert")).toBeInTheDocument();
  });

  it("reads a pasted figure's zero cents on a no-decimal field", async () => {
    const { onChange, input } = setup({ decimals: 0 });
    await userEvent.click(input);
    await userEvent.paste("1,000.00");
    expect(input.value).toBe("1,000");
    expect(onChange).toHaveBeenLastCalledWith("1000");
  });

  it("leaves a range selected before its caret frame alone, so select-then-type replaces", () => {
    // Playwright's fill() focuses, selects all, then types. A deferred caret
    // move landing between the select and the type would collapse the
    // selection and append (Uruni #429).
    const frames: FrameRequestCallback[] = [];
    const raf = vi.spyOn(window, "requestAnimationFrame").mockImplementation((cb) => {
      frames.push(cb);
      return frames.length;
    });
    try {
      setup({ initial: "50000" });
      const input = screen.getByLabelText("Amount") as HTMLInputElement;
      fireEvent.focus(input);
      input.setSelectionRange(0, input.value.length);
      act(() => frames.forEach((cb) => cb(0)));
      expect([input.selectionStart, input.selectionEnd]).toEqual([0, 6]);
    } finally {
      raf.mockRestore();
    }
  });

  it("moves a collapsed caret to the end once focused", () => {
    const frames: FrameRequestCallback[] = [];
    const raf = vi.spyOn(window, "requestAnimationFrame").mockImplementation((cb) => {
      frames.push(cb);
      return frames.length;
    });
    try {
      setup({ initial: "50000" });
      const input = screen.getByLabelText("Amount") as HTMLInputElement;
      fireEvent.focus(input);
      input.setSelectionRange(2, 2);
      act(() => frames.forEach((cb) => cb(0)));
      expect([input.selectionStart, input.selectionEnd]).toEqual([6, 6]);
    } finally {
      raf.mockRestore();
    }
  });

  it("follows a value set from outside while unfocused", () => {
    const { rerender } = render(
      <NumericInput aria-label="X" decimals={2} value="1000" onChange={() => {}} />,
    );
    const input = screen.getByLabelText("X") as HTMLInputElement;
    expect(input.value).toBe("1,000");
    rerender(<NumericInput aria-label="X" decimals={2} value="2500.5" onChange={() => {}} />);
    expect(input.value).toBe("2,500.5");
  });

  describe("allowNegative", () => {
    it("ignores a minus on a field that takes no negative", async () => {
      const { onChange, input } = setup();
      await userEvent.type(input, "-35");
      expect(input.value).toBe("35");
      expect(onChange).toHaveBeenLastCalledWith("35");
    });

    it("takes a leading minus and emits a signed canonical value", async () => {
      const { onChange, input } = setup({ allowNegative: true });
      await userEvent.type(input, "-1234,5");
      expect(input.value).toBe("-1,234.5");
      expect(onChange).toHaveBeenLastCalledWith("-1234.5");
    });

    it("emits nothing for a lone minus", async () => {
      const { onChange, input } = setup({ allowNegative: true });
      await userEvent.type(input, "-");
      expect(input.value).toBe("-");
      expect(onChange).toHaveBeenLastCalledWith("");
    });

    it("toggles the sign when a minus is typed anywhere, keeping the caret by its digit", async () => {
      const { onChange, input } = setup({ allowNegative: true, initial: "2.5" });
      await userEvent.type(input, "-", { initialSelectionStart: 3, initialSelectionEnd: 3 });
      expect(input.value).toBe("-2.5");
      expect(input.selectionStart).toBe(4);
      expect(onChange).toHaveBeenLastCalledWith("-2.5");
      await userEvent.type(input, "-", { initialSelectionStart: 4, initialSelectionEnd: 4 });
      expect(input.value).toBe("2.5");
      expect(onChange).toHaveBeenLastCalledWith("2.5");
    });

    it("drops the sign on backspace over it", async () => {
      const { onChange, input } = setup({ allowNegative: true, initial: "-3" });
      await userEvent.type(input, "{Backspace}", {
        initialSelectionStart: 1,
        initialSelectionEnd: 1,
      });
      expect(input.value).toBe("3");
      expect(onChange).toHaveBeenLastCalledWith("3");
    });

    it("reads a pasted negative", async () => {
      const { onChange, input } = setup({ allowNegative: true });
      await userEvent.click(input);
      await userEvent.paste("−1,250.75");
      expect(input.value).toBe("-1,250.75");
      expect(onChange).toHaveBeenLastCalledWith("-1250.75");
    });

    it("prefills a stored negative grouped", () => {
      setup({ allowNegative: true, initial: "-1500.5" });
      expect((screen.getByLabelText("Amount") as HTMLInputElement).value).toBe("-1,500.5");
    });
  });
});
