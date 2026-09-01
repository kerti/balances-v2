// AuthLayout is the shared pre-auth gate shell (ADR-0050 amendment). The
// divergence is single-tree: the desktop hero is additive brand chrome, so
// interactive/asserted children must render exactly once at every width — never
// duplicated across a render-both-and-hide split. These assertions pin that
// contract (jsdom has no viewport, so both the hero and the card content are in
// the DOM together — exactly the point: one tree, one testid).
import { it, expect } from "vitest";
import { screen } from "@testing-library/react";
import i18n from "@/i18n";
import { renderWithProviders } from "@/test/renderWithProviders";
import { ThemeProvider } from "@/theme/ThemeProvider";
import { AuthLayout } from "@/components/shell/AuthLayout";

it("renders interactive children exactly once alongside the brand hero", () => {
  renderWithProviders(
    <ThemeProvider>
      <AuthLayout>
        <div data-testid="gate-control" />
      </AuthLayout>
    </ThemeProvider>,
  );

  // The child (an interactive/asserted element) is single-instance — not
  // duplicated across a desktop/mobile split.
  expect(screen.getAllByTestId("gate-control")).toHaveLength(1);

  // The hero's brand headline (additive chrome, no testid) is present.
  expect(screen.getByText(i18n.t("common:authHero.headline"))).toBeInTheDocument();

  // The child sits in the main region, the hero in a complementary aside.
  expect(screen.getByRole("main")).toContainElement(screen.getByTestId("gate-control"));
  expect(screen.getByRole("complementary")).not.toContainElement(
    screen.getByTestId("gate-control"),
  );
});
