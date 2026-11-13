import { useState } from "react";
import { it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SegmentedControl } from "@/components/ui/segmented-control";

const OPTIONS = [
  { value: "a", label: "Alpha", testId: "seg-a" },
  { value: "b", label: "Beta", testId: "seg-b" },
  { value: "c", label: "Gamma", testId: "seg-c" },
];

function Harness({ onChange }: { onChange?: (v: string) => void }) {
  const [v, setV] = useState("a");
  return (
    <SegmentedControl
      ariaLabel="Pick"
      options={OPTIONS}
      value={v}
      onChange={(n) => {
        setV(n);
        onChange?.(n);
      }}
    />
  );
}

it("renders a radiogroup exposing the selected option", () => {
  render(<Harness />);
  expect(screen.getByRole("radiogroup", { name: "Pick" })).toBeInTheDocument();
  expect(screen.getAllByRole("radio")).toHaveLength(3);
  expect(screen.getByTestId("seg-a")).toHaveAttribute("aria-checked", "true");
  expect(screen.getByTestId("seg-b")).toHaveAttribute("aria-checked", "false");
});

it("selects on click and fires onChange", async () => {
  const onChange = vi.fn();
  render(<Harness onChange={onChange} />);
  await userEvent.click(screen.getByTestId("seg-b"));
  expect(onChange).toHaveBeenCalledWith("b");
  expect(screen.getByTestId("seg-b")).toHaveAttribute("aria-checked", "true");
});

it("moves and selects with arrow keys, wrapping, with one tab stop", async () => {
  const onChange = vi.fn();
  render(<Harness onChange={onChange} />);
  expect(screen.getByTestId("seg-a")).toHaveAttribute("tabindex", "0");
  expect(screen.getByTestId("seg-b")).toHaveAttribute("tabindex", "-1");
  screen.getByTestId("seg-a").focus();
  await userEvent.keyboard("{ArrowRight}");
  expect(onChange).toHaveBeenLastCalledWith("b");
  expect(screen.getByTestId("seg-b")).toHaveFocus();
  await userEvent.keyboard("{ArrowLeft}{ArrowLeft}");
  expect(onChange).toHaveBeenLastCalledWith("c");
  expect(screen.getByTestId("seg-c")).toHaveFocus();
});

it("keeps the first option tabbable when nothing is selected", () => {
  render(<SegmentedControl ariaLabel="Pick" options={OPTIONS} value={null} onChange={vi.fn()} />);
  expect(screen.getByTestId("seg-a")).toHaveAttribute("tabindex", "0");
  for (const r of screen.getAllByRole("radio")) expect(r).toHaveAttribute("aria-checked", "false");
});
