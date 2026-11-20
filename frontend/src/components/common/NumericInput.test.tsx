import { useState } from "react";
import { act, fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import i18n from "@/i18n";
import { NumericInput } from "@/components/common/NumericInput";
import { ValidatedForm } from "@/components/common/ValidatedForm";
import { currencyDecimals } from "@/lib/format";

// Ported from kerti/uruni's AmountInput tests (ADR-0056), widened from whole
// rupiah to a decimal model: digits plus at most one decimal point.

// NumericInput moves the caret to the end one frame after focus, so the
// caret is placed only after that frame has run — otherwise it races the
// typing (user-event's `initialSelectionStart` loses that race intermittently).
async function typeAt(input: HTMLInputElement, text: string, start: number, end = start) {
  if (document.activeElement !== input) {
    act(() => input.focus());
    await flushFrame();
  }
  input.setSelectionRange(start, end);
  await userEvent.keyboard(text);
}

function Controlled({
  onChange,
  decimals = 2,
  maxIntegerDigits,
  allowNegative,
  initial = "",
  required,
}: {
  onChange: (v: string) => void;
  required?: boolean;
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
        required={required}
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

  // ADR-0056 §2, kept on purpose when IDR gained cents (#787, 2026-11-05): on a
  // field that takes decimals, a typed separator is the decimal point, so the
  // en-GB habit of typing the grouping lands on 48.50. Live grouping shows it
  // before saving; a paste of the same text is read as grouping.
  it("reads a typed comma as the decimal point on an IDR amount", async () => {
    const { onChange, input } = setup({ decimals: currencyDecimals("IDR") });
    await userEvent.type(input, "48,500,000");
    expect(input.value).toBe("48.50");
    expect(onChange).toHaveBeenLastCalledWith("48.50");

    await userEvent.clear(input);
    await userEvent.paste("48,500,000");
    expect(input.value).toBe("48,500,000");
    expect(onChange).toHaveBeenLastCalledWith("48500000");
  });

  it("reads text inserted in one go (autofill, dictation, fill()) like a paste", () => {
    const { onChange, input } = setup({ decimals: currencyDecimals("IDR") });
    fireEvent.change(input, { target: { value: "48,500,000" } });
    expect(input.value).toBe("48,500,000");
    expect(onChange).toHaveBeenLastCalledWith("48500000");

    // Text that doesn't parse whole falls back to the per-key rule, unless it
    // misuses the grouping mark: that is refused (#933), never read as a decimal.
    onChange.mockClear();
    fireEvent.change(input, { target: { value: "48,500,0001,5" } });
    expect(onChange).not.toHaveBeenCalled();
    expect(input.value).toBe("48,500,000");
    expect(screen.getByRole("alert")).toHaveTextContent("Check this number");

    // A one-shot insert with no grouping mark in it is still the per-key rule.
    fireEvent.change(input, { target: { value: "48,500,0005a" } });
    expect(onChange).toHaveBeenLastCalledWith("485000005");
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
    await typeAt(input, "1", 1);
    expect(input.value).toBe("415,000");
    expect(input.selectionStart).toBe(2);

    // Backspace there takes the "1" back, caret after the "4".
    await typeAt(input, "{Backspace}", 2);
    expect(input.value).toBe("45,000");
    expect(input.selectionStart).toBe(1);
    expect(onChange).toHaveBeenLastCalledWith("45000");
  });

  it("takes the digit before a group separator on backspace", async () => {
    const { onChange, input } = setup();
    await userEvent.type(input, "45000");
    await flushFrame();
    // Caret just after the comma in 45,|000.
    await typeAt(input, "{Backspace}", 3);
    expect(input.value).toBe("4,000");
    expect(onChange).toHaveBeenLastCalledWith("4000");
  });

  it("takes the digit after a group separator on forward delete", async () => {
    const { onChange, input } = setup();
    await userEvent.type(input, "45000");
    await flushFrame();
    // Caret just before the comma in 45|,000.
    await typeAt(input, "{Delete}", 2);
    expect(input.value).toBe("4,500");
    expect(onChange).toHaveBeenLastCalledWith("4500");
  });

  it("replaces a selected value with what is typed", async () => {
    const { onChange, input } = setup({ initial: "50000" });
    expect(input.value).toBe("50,000");
    await typeAt(input, "23456", 0, input.value.length);
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
      await typeAt(input, "-", 3);
      expect(input.value).toBe("-2.5");
      expect(input.selectionStart).toBe(4);
      expect(onChange).toHaveBeenLastCalledWith("-2.5");
      await typeAt(input, "-", 4);
      expect(input.value).toBe("2.5");
      expect(onChange).toHaveBeenLastCalledWith("2.5");
    });

    it("drops the sign on backspace over it", async () => {
      const { onChange, input } = setup({ allowNegative: true, initial: "-3" });
      await typeAt(input, "{Backspace}", 1);
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

// #933: a keystroke the field drops says so, briefly, without refusing anything
// else, and without moving what is emitted.
// covers: INV-PRESENTATION-35
describe("NumericInput: Numbers only hint", () => {
  const hint = () => screen.getByRole("status");

  it("is silent until something is dropped, and the live region is always mounted", async () => {
    const { input } = setup();
    expect(hint()).toHaveTextContent("");
    await userEvent.type(input, "12");
    expect(hint()).toHaveTextContent("");
  });

  it.each([
    ["a letter", "a", "en-GB"],
    ["a space", " ", "en-GB"],
    ["a symbol", "%", "id-ID"],
    ["a minus on an unsigned field", "-", "en-GB"],
  ])("shows for %s, in the app language", async (_name, key, locale) => {
    await i18n.changeLanguage(locale);
    const { onChange, input } = setup();
    await userEvent.type(input, "5");
    onChange.mockClear();
    await userEvent.type(input, key);
    expect(hint()).toHaveTextContent(locale === "en-GB" ? "Numbers only" : "Hanya angka");
    expect(hint()).toHaveAttribute("aria-live", "polite");
    expect(input.value).toBe("5");
    expect(onChange).not.toHaveBeenCalled();
  });

  it.each([
    ["a digit", "5", {}],
    ["a typed separator", ".", {}],
    ["a typed comma", ",", {}],
    ["a separator on a no-decimal field", ",", { decimals: 0 }],
  ])("does not show for %s", async (_name, key, props) => {
    const { input } = setup(props);
    await userEvent.type(input, `1${key}`);
    expect(hint()).toHaveTextContent("");
  });

  it("does not show for a minus on a signed field", async () => {
    const { input } = setup({ allowNegative: true });
    await userEvent.type(input, "-5");
    expect(hint()).toHaveTextContent("");
  });

  it("goes after about two seconds, and a second dropped key restarts the clock", () => {
    vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout"] });
    try {
      const { input } = setup();
      fireEvent.change(input, { target: { value: "x" } });
      expect(hint()).toHaveTextContent("Numbers only");
      act(() => vi.advanceTimersByTime(1500));
      fireEvent.change(input, { target: { value: "xy" } });
      act(() => vi.advanceTimersByTime(1500));
      expect(hint()).toHaveTextContent("Numbers only");
      act(() => vi.advanceTimersByTime(600));
      expect(hint()).toHaveTextContent("");
    } finally {
      vi.useRealTimers();
    }
  });
});

// #933: grouping that is not thousands is refused, never guessed at.
// covers: INV-PRESENTATION-35
describe("NumericInput: malformed grouping", () => {
  const refused = [
    ["en-GB", "1,5"],
    ["en-GB", "12,34,567"],
    ["en-GB", "1234,567"],
    ["en-GB", "1,2345"],
    ["en-GB", "1,000,00"],
    ["en-GB", "1.000,5"],
    ["id-ID", "1.5"],
    ["id-ID", "12.34.567"],
    ["id-ID", "1234.567"],
    ["id-ID", "1.2345"],
    ["id-ID", "1,000.5"],
  ] as const;
  const read = [
    ["en-GB", "1,000", "1000"],
    ["en-GB", "1,234,567.89", "1234567.89"],
    ["en-GB", "999", "999"],
    ["id-ID", "1.000", "1000"],
    ["id-ID", "1.234.567,89", "1234567.89"],
    ["id-ID", "1,5", "1.5"],
    ["id-ID", "999", "999"],
  ] as const;
  const message = (locale: string) =>
    locale === "en-GB" ? "Check this number" : "Periksa angka ini";

  it.each(refused)("refuses a paste in %s of %s, keeping the value", async (locale, text) => {
    await i18n.changeLanguage(locale);
    const { onChange, input } = setup({ initial: "500" });
    await userEvent.click(input);
    await userEvent.paste(text);
    expect(input.value).toBe("500");
    expect(onChange).not.toHaveBeenCalled();
    expect(screen.getByRole("alert")).toHaveTextContent(message(locale));
  });

  it.each(refused)("refuses a one-shot insert in %s of %s", async (locale, text) => {
    await i18n.changeLanguage(locale);
    const { onChange, input } = setup();
    fireEvent.change(input, { target: { value: text } });
    expect(input.value).toBe("");
    expect(onChange).not.toHaveBeenCalled();
    expect(screen.getByRole("alert")).toHaveTextContent(message(locale));
  });

  it.each(read)("reads a well-formed paste in %s of %s", async (locale, text, want) => {
    await i18n.changeLanguage(locale);
    const { onChange, input } = setup();
    await userEvent.click(input);
    await userEvent.paste(text);
    expect(onChange).toHaveBeenLastCalledWith(want);
    expect(screen.queryByRole("alert")).toBeNull();
    expect(input.validity.valid).toBe(true);
  });

  it("does not call a different failure malformed grouping", async () => {
    const { input } = setup();
    await userEvent.click(input);
    await userEvent.paste("1.2.3");
    expect(screen.getByRole("alert")).toHaveTextContent("isn't a number");
    // Nor does it block Save: only a malformed figure does.
    expect(input.validity.valid).toBe(true);
  });

  it("clears on the next edit, and marks the field invalid while it stands", async () => {
    const { onChange, input } = setup({ initial: "500" });
    await userEvent.click(input);
    await userEvent.paste("1,5");
    expect(input).toHaveAttribute("aria-invalid", "true");
    expect(input.validity.valid).toBe(false);
    expect(input.validationMessage).toBe("Check this number");

    await userEvent.type(input, "0");
    expect(onChange).toHaveBeenLastCalledWith("5000");
    expect(screen.queryByRole("alert")).toBeNull();
    expect(input.validity.valid).toBe(true);
  });

  it("clears when the value is set from outside", () => {
    const { rerender } = render(
      <NumericInput aria-label="X" decimals={2} value="500" onChange={() => {}} />,
    );
    const input = screen.getByLabelText("X") as HTMLInputElement;
    fireEvent.paste(input, { clipboardData: { getData: () => "1,5" } });
    expect(input.validity.valid).toBe(false);
    rerender(<NumericInput aria-label="X" decimals={2} value="900" onChange={() => {}} />);
    expect(input.validity.valid).toBe(true);
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("keeps the typed-separator rule: a keyed comma is still the decimal point", async () => {
    const { onChange, input } = setup();
    await userEvent.type(input, "1,5");
    expect(input.value).toBe("1.5");
    expect(onChange).toHaveBeenLastCalledWith("1.5");
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("blocks a ValidatedForm's submit and says why, once", async () => {
    const onSubmit = vi.fn();
    render(
      <ValidatedForm onSubmit={onSubmit}>
        <Controlled onChange={() => {}} initial="500" />
        <button type="submit">Save</button>
      </ValidatedForm>,
    );
    const input = screen.getByLabelText("Amount") as HTMLInputElement;
    await userEvent.click(input);
    await userEvent.paste("12,34,567");
    await userEvent.click(screen.getByRole("button", { name: "Save" }));

    expect(onSubmit).not.toHaveBeenCalled();
    expect(screen.getAllByText("Check this number")).toHaveLength(1);
    expect(screen.getByTestId("field-error-amt")).toHaveTextContent("Check this number");

    // Fixing the figure lets Save through, with the message gone.
    await userEvent.type(input, "1");
    expect(screen.queryByText("Check this number")).toBeNull();
    await userEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(onSubmit).toHaveBeenCalledTimes(1);
  });

  it("names the malformed figure, not 'required', on an empty required field", async () => {
    render(
      <ValidatedForm onSubmit={vi.fn()}>
        <Controlled onChange={() => {}} required />
        <button type="submit">Save</button>
      </ValidatedForm>,
    );
    const input = screen.getByLabelText("Amount") as HTMLInputElement;
    await userEvent.click(input);
    await userEvent.paste("1,5");
    await userEvent.click(screen.getByRole("button", { name: "Save" }));

    expect(screen.getByTestId("field-error-amt")).toHaveTextContent("Check this number");
    expect(screen.getAllByText("Check this number")).toHaveLength(1);
    expect(screen.queryByText("This field is required.")).toBeNull();
  });
});
