// covers: INV-PRESENTATION-23
import { describe, expect, it } from "vitest";
import { screen } from "@testing-library/react";
import { renderWithProviders } from "@/test/renderWithProviders";
import { StatusBadge } from "@/components/common/StatusBadge";

describe("StatusBadge", () => {
  it("shows Active as a neutral outline pill, not green", () => {
    renderWithProviders(<StatusBadge group="assets" status="active" />);
    const pill = screen.getByTestId("status-badge");
    expect(pill).toHaveTextContent("Active");
    expect(pill.className).toContain("border-border");
    expect(pill.className).toContain("text-foreground");
    expect(pill.className).not.toMatch(/green|emerald/);
  });

  it("keeps the muted pill for a terminal status", () => {
    renderWithProviders(<StatusBadge group="assets" status="closed" />);
    const pill = screen.getByTestId("status-badge");
    expect(pill).toHaveTextContent("Closed");
    expect(pill.className).toContain("bg-muted");
    expect(pill.className).toContain("text-muted-foreground");
  });
});
