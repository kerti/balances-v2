import { describe, expect, it } from "vitest";
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithProviders } from "@/test/renderWithProviders";
import { CreateIncomeDialog } from "@/components/dialogs/CreateIncomeDialog";

function regularityRadios() {
  const [routine, incidental] = screen.getAllByRole("radio") as HTMLInputElement[];
  return { routine, incidental };
}

describe("CreateIncomeDialog regularity default (#953 S7)", () => {
  it("pre-picks Regular for Salary and Pension, One-off for Gift", async () => {
    renderWithProviders(<CreateIncomeDialog open onOpenChange={() => {}} hideTrigger />);
    const category = await screen.findByLabelText("Category");

    await userEvent.selectOptions(category, "pension");
    expect(regularityRadios().routine).toBeChecked();

    await userEvent.selectOptions(category, "gift");
    expect(regularityRadios().incidental).toBeChecked();

    await userEvent.selectOptions(category, "salary");
    expect(regularityRadios().routine).toBeChecked();
  });

  it("keeps a hand-picked regularity when the category changes afterwards", async () => {
    renderWithProviders(<CreateIncomeDialog open onOpenChange={() => {}} hideTrigger />);
    const category = await screen.findByLabelText("Category");

    await userEvent.click(regularityRadios().incidental);
    await userEvent.selectOptions(category, "salary");
    expect(regularityRadios().incidental).toBeChecked();
  });
});
