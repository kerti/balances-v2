// The inline-validation mechanism (#824, ADR-0058 §5): per-field messages on
// blur and on submit, a submit that is never disabled for an invalid form, and
// server error codes drawn beside their field (or as one form-level message).
import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import i18n from "@/i18n";
import { ApiError } from "@/api/client";
import { FormError, ValidatedForm } from "@/components/common/ValidatedForm";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";

const required = i18n.t("common:validation.required");

function Harness({
  onSubmit = () => {},
  error = null,
  validate,
}: {
  onSubmit?: () => void;
  error?: unknown;
  validate?: () => Record<string, string>;
}) {
  return (
    <ValidatedForm onSubmit={onSubmit} error={error} validate={validate} data-testid="form">
      <label htmlFor="display_name">Name</label>
      <Input id="display_name" required defaultValue="" />
      <label htmlFor="bank_name">Bank</label>
      <Input id="bank_name" required defaultValue="" />
      <label htmlFor="kind">Kind</label>
      <Select id="kind" required defaultValue="">
        <option value="">Pick one</option>
        <option value="a">A</option>
      </Select>
      <label htmlFor="year_month">Month</label>
      <Input id="year_month" defaultValue="2026-01" />
      <FormError />
      <button type="submit">Save</button>
    </ValidatedForm>
  );
}

// covers: INV-PRESENTATION-19
describe("ValidatedForm", () => {
  it("turns off the browser's own bubbles", () => {
    render(<Harness />);
    expect(screen.getByTestId("form")).toHaveAttribute("novalidate");
  });

  it("an empty submit lists every missing field at once and does not submit", async () => {
    const onSubmit = vi.fn();
    render(<Harness onSubmit={onSubmit} />);
    const save = screen.getByRole("button", { name: "Save" });
    expect(save).toBeEnabled();

    await userEvent.click(save);

    expect(onSubmit).not.toHaveBeenCalled();
    expect(screen.getByTestId("field-error-display_name")).toHaveTextContent(required);
    expect(screen.getByTestId("field-error-bank_name")).toHaveTextContent(required);
    expect(screen.getByTestId("field-error-kind")).toHaveTextContent(required);
    expect(screen.queryByTestId("field-error-year_month")).not.toBeInTheDocument();
    const name = screen.getByLabelText("Name");
    expect(name).toHaveAttribute("aria-invalid", "true");
    expect(name).toHaveAttribute("aria-describedby", "display_name-error");
    // The first invalid field takes focus.
    expect(name).toHaveFocus();
    expect(save).toBeEnabled();
  });

  it("shows a field's message on blur and clears it once the field is fixed", async () => {
    render(<Harness />);
    await userEvent.click(screen.getByLabelText("Name"));
    await userEvent.tab();
    expect(screen.getByTestId("field-error-display_name")).toBeInTheDocument();
    // Only the field that was left shows anything.
    expect(screen.queryByTestId("field-error-bank_name")).not.toBeInTheDocument();

    await userEvent.type(screen.getByLabelText("Name"), "x");
    expect(screen.queryByTestId("field-error-display_name")).not.toBeInTheDocument();
    expect(screen.getByLabelText("Name")).not.toHaveAttribute("aria-invalid", "true");
  });

  it("submits once every field is valid", async () => {
    const onSubmit = vi.fn();
    render(<Harness onSubmit={onSubmit} />);
    await userEvent.type(screen.getByLabelText("Name"), "a");
    await userEvent.type(screen.getByLabelText("Bank"), "b");
    await userEvent.selectOptions(screen.getByLabelText("Kind"), "a");
    await userEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(onSubmit).toHaveBeenCalledTimes(1);
  });

  it("shows a custom rule on submit and keeps submit enabled", async () => {
    const onSubmit = vi.fn();
    render(<Harness onSubmit={onSubmit} validate={() => ({ year_month: "Not that month" })} />);
    await userEvent.type(screen.getByLabelText("Name"), "a");
    await userEvent.type(screen.getByLabelText("Bank"), "b");
    await userEvent.selectOptions(screen.getByLabelText("Kind"), "a");
    await userEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(onSubmit).not.toHaveBeenCalled();
    expect(screen.getByTestId("field-error-year_month")).toHaveTextContent("Not that month");
  });

  it("draws a server code that belongs to a field beside it, not as a form-level message", () => {
    const error = new ApiError(409, "conflict", { code: "SNAPSHOT_MONTH_EXISTS" });
    render(<Harness error={error} />);
    expect(screen.getByTestId("field-error-year_month")).toHaveTextContent(
      i18n.t("errors:code.SNAPSHOT_MONTH_EXISTS"),
    );
    expect(screen.queryByTestId("form-error")).not.toBeInTheDocument();
  });

  it("draws a server code with no field as one form-level message", () => {
    const error = new ApiError(500, "boom", { code: "INTERNAL" });
    render(<Harness error={error} />);
    expect(screen.getByTestId("form-error")).toHaveTextContent(i18n.t("errors:code.INTERNAL"));
    expect(screen.queryByTestId("field-error-year_month")).not.toBeInTheDocument();
  });

  it("retires the server message when the user edits a field", async () => {
    const error = new ApiError(409, "conflict", { code: "SNAPSHOT_MONTH_EXISTS" });
    render(<Harness error={error} />);
    await userEvent.type(screen.getByLabelText("Month"), "1");
    expect(screen.queryByTestId("field-error-year_month")).not.toBeInTheDocument();
  });
});
