// Contract test for the shared InfoGrid primitive (ADR-0051). It renders nothing
// for an empty field list (an amount-only type) and lays out label/value pairs
// where the value is an already-formatted node — the grid never inspects field
// identity, so a right-aligned numeric column comes from a node-level class the
// primitive stays blind to.
import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { InfoGrid } from "./InfoGrid";

describe("InfoGrid", () => {
  it("renders nothing for an empty field list", () => {
    const { container } = render(<InfoGrid fields={[]} />);
    expect(container).toBeEmptyDOMElement();
  });

  it("lays out each label/value pair with the value node verbatim", () => {
    render(
      <InfoGrid
        fields={[
          { label: "Interest rate", value: "5.5%" },
          { label: "Balance", value: <span className="ml-auto tabular-nums">{"1,234"}</span> },
        ]}
      />,
    );
    expect(screen.getByText("Interest rate")).toBeInTheDocument();
    expect(screen.getByText("5.5%")).toBeInTheDocument();
    // Node-level alignment rides on the value, not a grid-read flag.
    expect(screen.getByText("1,234")).toHaveClass("ml-auto");
  });
});
