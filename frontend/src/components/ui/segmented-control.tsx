import { useRef, type KeyboardEvent, type ReactNode } from "react";
import { cn } from "@/lib/utils";

export type SegmentedOption<T extends string> = {
  value: T;
  label: ReactNode;
  testId?: string;
  title?: string;
};

type Props<T extends string> = {
  options: SegmentedOption<T>[];
  /** null = nothing selected (e.g. a chart range that matches no preset). */
  value: T | null;
  onChange: (next: T) => void;
  ariaLabel: string;
  testId?: string;
  className?: string;
};

// "Pick one of a few" (#897): one bordered track, the selected segment raised on
// a neutral surface, the rest plain muted text. Deliberately never the brand
// `primary` fill, so it reads as a setting and the page's primary action stays
// the only gold thing. A radiogroup: arrow keys move AND select, one tab stop.
export function SegmentedControl<T extends string>({
  options,
  value,
  onChange,
  ariaLabel,
  testId,
  className,
}: Props<T>) {
  const refs = useRef<(HTMLButtonElement | null)[]>([]);
  const selectedIdx = options.findIndex((o) => o.value === value);
  const tabIdx = selectedIdx === -1 ? 0 : selectedIdx;

  const move = (to: number) => {
    const n = options.length;
    const idx = ((to % n) + n) % n;
    onChange(options[idx].value);
    refs.current[idx]?.focus();
  };

  const onKeyDown = (e: KeyboardEvent, i: number) => {
    switch (e.key) {
      case "ArrowRight":
      case "ArrowDown":
        e.preventDefault();
        move(i + 1);
        break;
      case "ArrowLeft":
      case "ArrowUp":
        e.preventDefault();
        move(i - 1);
        break;
      case "Home":
        e.preventDefault();
        move(0);
        break;
      case "End":
        e.preventDefault();
        move(options.length - 1);
        break;
    }
  };

  return (
    <div
      role="radiogroup"
      aria-label={ariaLabel}
      data-testid={testId}
      className={cn(
        "inline-flex gap-0.5 rounded-lg border bg-muted p-0.5 max-md:flex max-md:w-full",
        className,
      )}
    >
      {options.map((o, i) => {
        const on = i === selectedIdx;
        return (
          <button
            key={o.value}
            ref={(el) => {
              refs.current[i] = el;
            }}
            type="button"
            role="radio"
            aria-checked={on}
            tabIndex={i === tabIdx ? 0 : -1}
            title={o.title}
            data-testid={o.testId}
            onClick={() => onChange(o.value)}
            onKeyDown={(e) => onKeyDown(e, i)}
            className={cn(
              "inline-flex items-center justify-center rounded-md px-3 text-[0.8rem] font-medium whitespace-nowrap outline-none transition-colors",
              "h-7 max-md:min-h-11 max-md:flex-1 focus-visible:ring-3 focus-visible:ring-ring/50",
              on
                ? "bg-background text-foreground shadow-sm"
                : "text-muted-foreground hover:text-foreground",
            )}
          >
            {o.label}
          </button>
        );
      })}
    </div>
  );
}
