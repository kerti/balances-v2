import {
  useId,
  useLayoutEffect,
  useRef,
  useState,
  type ChangeEvent,
  type ClipboardEvent,
  type ComponentProps,
  type FocusEvent,
} from "react";
import { useTranslation } from "react-i18next";
import { Input } from "@/components/ui/input";
import { groupCanonical, numberSeparators, parseLocaleNumber } from "@/lib/format";

// NumericInput is the one field for amounts, quantities, prices and rates
// (ADR-0056), ported from kerti/uruni's AmountInput. It holds and emits a
// canonical decimal string ("48500000.50", the wire format) and owns only how
// that string is displayed: grouped live in the app locale while you type.
//
// The model is digits plus at most one decimal point, after a leading "-" on a
// field that takes negatives. Because the field inserts
// the grouping itself, a typed "." or "," can only mean the decimal point, which
// is what makes a phone's region-following decimal key work in either locale.
// Pasted text goes through the strict parseLocaleNumber instead.
type Props = Omit<ComponentProps<typeof Input>, "value" | "onChange" | "type" | "inputMode"> & {
  value: string;
  onChange: (value: string) => void;
  // Decimal places a typed or pasted figure may carry. A stored value with more
  // keeps them (it arrived that way; the edit shouldn't silently round it).
  decimals: number;
  // Integer digits the column holds: 16 for NUMERIC(20,4), 12 for (20,8).
  maxIntegerDigits?: number;
  // A signed figure (an appreciation or inflation rate). A typed minus toggles
  // the sign wherever the caret is.
  allowNegative?: boolean;
};

type PasteError = "invalid" | "tooLong";

const MODEL_RE = /^-?\d*(\.\d*)?$/;

// The model a stored value starts from. Anything that isn't one (it shouldn't
// happen; values come from the wire) is shown raw rather than mangled.
function modelOf(value: string): string {
  return MODEL_RE.test(value) ? value : "";
}

// The model emitted on the wire: a half-typed "12." is sent as "12", a lone
// "-" as nothing.
function canonical(model: string): string {
  const out = model.endsWith(".") ? model.slice(0, -1) : model;
  return out === "-" ? "" : out;
}

function fracLen(model: string): number {
  const dot = model.indexOf(".");
  return dot === -1 ? 0 : model.length - dot - 1;
}

function intLen(model: string): number {
  const body = model.startsWith("-") ? model.slice(1) : model;
  const dot = body.indexOf(".");
  return dot === -1 ? body.length : dot;
}

// normalize drops leading zeros ("05" reads as 5) and puts a 0 before a leading
// decimal point, returning how far the caret moves with it.
function normalize(model: string): { model: string; shift: number } {
  const sign = model.startsWith("-") ? "-" : "";
  const body = model.slice(sign.length);
  const trimmed = body.replace(/^0+(?=\d)/, "");
  const shift = trimmed.length - body.length;
  if (trimmed.startsWith(".")) return { model: sign + "0" + trimmed, shift: shift + 1 };
  return { model: sign + trimmed, shift };
}

export function NumericInput({
  value,
  onChange,
  decimals,
  maxIntegerDigits = 16,
  allowNegative = false,
  className,
  onFocus,
  onBlur,
  "aria-describedby": describedBy,
  ...rest
}: Props) {
  // Subscribes to language changes, so a locale switch re-renders the marks.
  const { t } = useTranslation("common");
  const { group, decimal } = numberSeparators();
  const errorId = useId();
  const inputRef = useRef<HTMLInputElement>(null);
  const [focused, setFocused] = useState(false);
  // What the user has typed while focused. Differs from `value` only by a
  // trailing "." (canonical() drops it); a value set from outside wins.
  const [model, setModel] = useState("");
  const [error, setError] = useState<PasteError | null>(null);
  // Model characters before the caret after the next render, set by an edit.
  const caretModel = useRef<number | null>(null);

  const shownModel = focused && canonical(model) === value ? model : modelOf(value);
  const display = MODEL_RE.test(value) || focused ? groupCanonical(shownModel) : value;

  // Display text → model characters: digits and the sign stay, the locale's
  // decimal mark is the point, a group mark is dropped.
  function toModel(text: string): string {
    let out = "";
    for (const ch of text) {
      if ((ch >= "0" && ch <= "9") || ch === "-") out += ch;
      else if (ch === decimal) out += ".";
    }
    return out;
  }

  // The display index just after the `count`-th model character.
  function displayIndex(text: string, count: number): number {
    if (count === 0) return 0;
    let seen = 0;
    for (let i = 0; i < text.length; i++) {
      if (text[i] === group) continue;
      seen += 1;
      if (seen === count) return i + 1;
    }
    return text.length;
  }

  useLayoutEffect(() => {
    const input = inputRef.current;
    if (caretModel.current === null || input === null) return;
    const pos = displayIndex(input.value, caretModel.current);
    caretModel.current = null;
    input.setSelectionRange(pos, pos);
  });

  // fits reports whether a candidate model is within the field's limits. Limits
  // only bind on growth, so a stored value past the default stays editable.
  function fits(next: string, prev: string): boolean {
    if (!MODEL_RE.test(next)) return false;
    if (fracLen(next) > Math.max(decimals, fracLen(prev))) return false;
    if (decimals === 0 && next.includes(".") && !prev.includes(".")) return false;
    return intLen(next) <= Math.max(maxIntegerDigits, intLen(prev));
  }

  function commit(next: string, caret: number) {
    const n = normalize(next);
    caretModel.current = Math.max(0, caret + n.shift);
    setModel(n.model);
    setError(null);
    onChange(canonical(n.model));
  }

  function handleChange(event: ChangeEvent<HTMLInputElement>) {
    const input = event.target;
    const prev = display;
    const next = input.value;
    // The edit is whatever differs between the rendered text and the new text.
    let p = 0;
    while (p < prev.length && p < next.length && prev[p] === next[p]) p++;
    let s = 0;
    while (
      s < prev.length - p &&
      s < next.length - p &&
      prev[prev.length - 1 - s] === next[next.length - 1 - s]
    )
      s++;
    const removed = prev.slice(p, prev.length - s);
    const inserted = next.slice(p, next.length - s);
    let head = toModel(prev.slice(0, p));
    let tail = toModel(prev.slice(prev.length - s));
    let typed = "";
    let flipSign = false;
    for (const ch of inserted) {
      if (ch >= "0" && ch <= "9") typed += ch;
      else if ((ch === "." || ch === ",") && decimals > 0) typed += ".";
      else if ((ch === "-" || ch === "\u2212") && allowNegative) flipSign = !flipSign;
    }
    // A delete that took only a group mark meant the digit beside it.
    if (inserted === "" && removed !== "" && toModel(removed) === "") {
      const inputType = (event.nativeEvent as InputEvent).inputType;
      if (inputType === "deleteContentBackward") head = head.slice(0, -1);
      else if (inputType === "deleteContentForward") tail = tail.slice(1);
    }
    let candidate = head + typed + tail;
    let caret = head.length + typed.length;
    if (flipSign) {
      const negative = candidate.startsWith("-");
      candidate = negative ? candidate.slice(1) : "-" + candidate;
      caret = Math.max(0, caret + (negative ? -1 : 1));
    }
    const prevModel = toModel(prev);
    if (candidate === prevModel || !fits(candidate, prevModel)) {
      // Refused: put the text back as it was, caret where the edit started.
      input.value = prev;
      input.setSelectionRange(p, p);
      return;
    }
    commit(candidate, caret);
  }

  function handlePaste(event: ClipboardEvent<HTMLInputElement>) {
    event.preventDefault();
    const input = event.currentTarget;
    const parsed = parseLocaleNumber(event.clipboardData.getData("text"), undefined, {
      allowNegative,
    });
    if (parsed === null) {
      setError("invalid");
      return;
    }
    // Zero cents on a field that takes fewer decimals are no change in value.
    let pasted = parsed;
    if (fracLen(pasted) > decimals) {
      pasted = pasted.replace(/0+$/, "");
      if (pasted.endsWith(".")) pasted = pasted.slice(0, -1);
    }
    const start = input.selectionStart ?? display.length;
    const end = input.selectionEnd ?? display.length;
    const head = toModel(display.slice(0, start));
    const candidate = head + pasted + toModel(display.slice(end));
    // A signed paste can only land at the start of the figure.
    if (!MODEL_RE.test(candidate)) {
      setError("invalid");
      return;
    }
    if (!fits(candidate, "")) {
      setError("tooLong");
      return;
    }
    commit(candidate, head.length + pasted.length);
  }

  function handleFocus(event: FocusEvent<HTMLInputElement>) {
    setModel(modelOf(value));
    setFocused(true);
    // Caret at the end, not where the tap landed — a frame later, so it lands
    // after the tap's own caret placement — and only on a collapsed caret: a
    // range selected in that frame (select-all, or Playwright's fill(), which
    // selects then types) is left alone, or the typing would append instead of
    // replacing (Uruni #429).
    const input = event.target;
    requestAnimationFrame(() => {
      if (input.selectionStart !== input.selectionEnd) return;
      input.setSelectionRange(input.value.length, input.value.length);
    });
    onFocus?.(event);
  }

  function handleBlur(event: FocusEvent<HTMLInputElement>) {
    setFocused(false);
    onBlur?.(event);
  }

  return (
    <div className="grid min-w-0 gap-1">
      <Input
        {...rest}
        ref={inputRef}
        type="text"
        // A phone's decimal keypad has no minus key, so a signed field gets the
        // full keyboard.
        inputMode={allowNegative ? "text" : decimals > 0 ? "decimal" : "numeric"}
        autoComplete="off"
        className={className}
        value={display}
        onChange={handleChange}
        onPaste={handlePaste}
        onFocus={handleFocus}
        onBlur={handleBlur}
        aria-invalid={error !== null || rest["aria-invalid"] ? true : undefined}
        aria-describedby={error ? [describedBy, errorId].filter(Boolean).join(" ") : describedBy}
      />
      {error && (
        <p id={errorId} role="alert" className="text-xs text-destructive">
          {t(error === "invalid" ? "numericInput.invalidPaste" : "numericInput.tooLong")}
        </p>
      )}
    </div>
  );
}
