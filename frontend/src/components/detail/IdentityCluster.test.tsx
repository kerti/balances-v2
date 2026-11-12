// The details card's identity line (#839): muted, dot-separated, and a missing
// piece takes its dot with it.
import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { IdentityCluster } from "./IdentityCluster";

describe("IdentityCluster", () => {
  it("joins the pieces on one muted line with dots", () => {
    render(<IdentityCluster lines={["Test Bank", "1234", "Savings"]} />);
    const line = screen.getByTestId("identity-line");
    expect(line).toHaveTextContent("Test Bank · 1234 · Savings");
    expect(line).toHaveClass("text-muted-foreground");
  });

  it("drops empty pieces without leaving a stray dot", () => {
    render(<IdentityCluster lines={["Bar", null, "24K", ""]} />);
    expect(screen.getByTestId("identity-line")).toHaveTextContent(/^Bar · 24K$/);
  });

  it("renders nothing when every piece is empty", () => {
    const { container } = render(<IdentityCluster lines={[null, undefined, ""]} />);
    expect(container).toBeEmptyDOMElement();
  });
});
