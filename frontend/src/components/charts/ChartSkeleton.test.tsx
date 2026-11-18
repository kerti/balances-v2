import { describe, expect, it } from "vitest";
import { screen } from "@testing-library/react";
import { renderWithProviders } from "@/test/renderWithProviders";
import { SnapshotChart } from "@/components/charts/SnapshotChart";
import { InvestmentPieChart } from "@/components/charts/InvestmentPieChart";

const snapshots = [
  { year_month: "2024-01", amount: "100" },
  { year_month: "2024-02", amount: "120" },
];

describe("chart loading skeleton", () => {
  it("SnapshotChart shows a sized skeleton while the lazy chart is pending", () => {
    renderWithProviders(<SnapshotChart snapshots={snapshots} currency="IDR" />);
    const sk = screen.getByTestId("chart-skeleton");
    expect(sk).toHaveClass("h-64", "w-full", "motion-reduce:animate-none");
  });

  it("InvestmentPieChart shows a skeleton matching the legend-position height", () => {
    renderWithProviders(
      <InvestmentPieChart
        slices={[{ key: "a", label: "A", value: 1, color: "red" }]}
        currency="IDR"
        legendPosition="none"
      />,
    );
    expect(screen.getByTestId("chart-skeleton")).toHaveClass("h-52");
  });
});
