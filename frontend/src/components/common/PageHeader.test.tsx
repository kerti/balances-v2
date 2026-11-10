import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { PageHeader } from "@/components/common/PageHeader";

describe("PageHeader", () => {
  it("renders title, subtitle and actions for a hub", () => {
    render(
      <PageHeader
        variant="hub"
        title="Income"
        subtitle="By month"
        actions={<button>New</button>}
      />,
    );
    expect(screen.getByRole("heading", { level: 1, name: "Income" })).toBeInTheDocument();
    expect(screen.getByText("By month")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "New" })).toBeInTheDocument();
  });

  it("renders a link back for a sub-page", () => {
    render(
      <MemoryRouter>
        <PageHeader variant="sub-page" back={{ label: "← Settings", to: "/settings" }} title="X" />
      </MemoryRouter>,
    );
    expect(screen.getByRole("link", { name: "← Settings" })).toHaveAttribute("href", "/settings");
  });

  it("renders a button back that calls onClick", async () => {
    const onClick = vi.fn();
    render(<PageHeader variant="sub-page" back={{ label: "← Back", onClick }} title="X" />);
    await userEvent.click(screen.getByRole("button", { name: "← Back" }));
    expect(onClick).toHaveBeenCalledOnce();
  });
});
