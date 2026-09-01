// Component test for the sign-in screen's public-demo notice (ADR-0041,
// #346/CF-03). The demo is a shared, public, writable sandbox behind one login;
// the notice is the documented acceptance of that posture (restore-commit stays
// live on demo, so the defacement-until-nightly-reset window is disclosed, not
// blocked). It renders only when the methods endpoint reports demo_mode — the
// same gate the pre-fill hint (LocalAuthForm) rides on.
import { describe, it, expect, afterEach } from "vitest";
import { http, HttpResponse } from "msw";
import { screen } from "@testing-library/react";
import i18n from "@/i18n";
import { renderWithProviders } from "@/test/renderWithProviders";
import { server } from "@/test/server";
import { ThemeProvider } from "@/theme/ThemeProvider";
import { SignInScreen } from "@/components/screens/SignInScreen";

const noticeTitle = i18n.t("common:signIn.demoNotice.title");

const originalWidth = window.innerWidth;
function setViewport(width: number) {
  Object.defineProperty(window, "innerWidth", { configurable: true, value: width });
}
afterEach(() => setViewport(originalWidth));

function useDemoMode() {
  server.use(
    http.get("/api/auth/methods", () =>
      HttpResponse.json({
        google: false,
        local: true,
        password_reset: false,
        demo_mode: true,
        demo_email: "demo@example.test",
        demo_password: "demo-password",
      }),
    ),
  );
}

describe("SignInScreen demo notice", () => {
  // covers: INV-AUTH-30
  it("shows the demo notice when the instance is in demo mode", async () => {
    server.use(
      http.get("/api/auth/methods", () =>
        HttpResponse.json({
          google: false,
          local: true,
          password_reset: false,
          demo_mode: true,
          demo_email: "demo@example.test",
          demo_password: "demo-password",
        }),
      ),
    );

    renderWithProviders(
      <ThemeProvider>
        <SignInScreen />
      </ThemeProvider>,
    );

    const notice = await screen.findByTestId("signin-demo-notice");
    expect(notice).toHaveTextContent(noticeTitle);
    // Each disclosed term is present, not just the title.
    expect(notice).toHaveTextContent(i18n.t("common:signIn.demoNotice.wipe"));
    expect(notice).toHaveTextContent(i18n.t("common:signIn.demoNotice.warranty"));
    expect(notice).toHaveTextContent(i18n.t("common:signIn.demoNotice.reset"));
  });

  // covers: INV-AUTH-30
  it("hides the demo notice on an ordinary (non-demo) instance", async () => {
    server.use(
      http.get("/api/auth/methods", () =>
        HttpResponse.json({
          google: true,
          local: false,
          password_reset: false,
          demo_mode: false,
        }),
      ),
    );

    renderWithProviders(
      <ThemeProvider>
        <SignInScreen />
      </ThemeProvider>,
    );

    // Wait for a provider affordance to prove methods resolved before asserting
    // the notice's absence (otherwise it's absent merely because nothing loaded).
    await screen.findByTestId("signin-google");
    expect(screen.queryByTestId("signin-demo-notice")).not.toBeInTheDocument();
  });

  // ADR-0050 amendment: the notice changes container by breakpoint via the
  // useIsMobile mount-split, so exactly one instance mounts — in the Card body
  // on a phone, in the hero (aside) on desktop — never duplicated.
  it("mounts the demo notice in the Card body on mobile (single instance)", async () => {
    setViewport(500);
    useDemoMode();
    renderWithProviders(
      <ThemeProvider>
        <SignInScreen />
      </ThemeProvider>,
    );
    const notice = await screen.findByTestId("signin-demo-notice");
    expect(screen.getAllByTestId("signin-demo-notice")).toHaveLength(1);
    expect(screen.getByRole("main")).toContainElement(notice);
  });

  it("mounts the demo notice in the hero column on desktop (single instance)", async () => {
    setViewport(1280);
    useDemoMode();
    renderWithProviders(
      <ThemeProvider>
        <SignInScreen />
      </ThemeProvider>,
    );
    const notice = await screen.findByTestId("signin-demo-notice");
    expect(screen.getAllByTestId("signin-demo-notice")).toHaveLength(1);
    expect(screen.getByRole("complementary")).toContainElement(notice);
  });
});
