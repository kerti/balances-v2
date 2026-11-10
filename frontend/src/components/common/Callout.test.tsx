import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Callout } from "./Callout";

describe("Callout", () => {
  it("defaults to the info variant", () => {
    render(<Callout data-testid="c">hello</Callout>);
    const el = screen.getByTestId("c");
    expect(el.dataset.variant).toBe("info");
    expect(el.className).toContain("bg-muted");
  });

  it("renders the warning variant on the warning tokens", () => {
    render(
      <Callout variant="warning" data-testid="c" className="mx-6">
        careful
      </Callout>,
    );
    const el = screen.getByTestId("c");
    expect(el.dataset.variant).toBe("warning");
    expect(el.className).toContain("bg-warning-surface");
    expect(el.className).toContain("mx-6");
    expect(el.getAttribute("role")).toBe("alert");
  });
});
