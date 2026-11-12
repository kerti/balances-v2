// Component test for the sign-in screen's public-demo notice (ADR-0041,
// #346/CF-03). The demo is a shared, public, writable sandbox behind one login;
// the notice is the documented acceptance of that posture (restore-commit stays
// live on demo, so the defacement-until-nightly-reset window is disclosed, not
// blocked). It renders only when the methods endpoint reports demo_mode — the
// same gate the pre-fill hint (LocalAuthForm) rides on.
import { describe, it, expect, afterEach } from "vitest";
import { http, HttpResponse } from "msw";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
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
        founding_open: true,
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
          founding_open: true,
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
          founding_open: true,
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

function useMethods(over: Record<string, unknown>) {
  server.use(
    http.get("/api/auth/methods", () =>
      HttpResponse.json({
        google: true,
        local: true,
        password_reset: false,
        demo_mode: false,
        founding_open: true,
        ...over,
      }),
    ),
  );
}

function renderSignIn() {
  return renderWithProviders(
    <ThemeProvider>
      <SignInScreen />
    </ThemeProvider>,
  );
}

describe("SignInScreen card (#842)", () => {
  // covers: INV-AUTH-33
  it("shows the Create account tab when founding is open", async () => {
    useMethods({ founding_open: true });
    renderSignIn();
    expect(await screen.findByTestId("local-mode-register")).toBeInTheDocument();
  });

  // covers: INV-AUTH-33
  it("hides the Create account tab when founding is closed", async () => {
    useMethods({ founding_open: false });
    renderSignIn();
    await screen.findByTestId("local-auth-form");
    expect(screen.queryByTestId("local-mode-register")).not.toBeInTheDocument();
    expect(screen.queryByTestId("local-display-name")).not.toBeInTheDocument();
    expect(screen.getByTestId("local-submit")).toBeInTheDocument();
  });

  it("demo mode: the name field shares the demo hint's cell, hidden and inert on Sign in", async () => {
    useMethods({ founding_open: true, demo_mode: true, demo_email: "d@x", demo_password: "p" });
    renderSignIn();
    const user = userEvent.setup();
    const field = (await screen.findByTestId("local-display-name")).parentElement!;
    expect(field).toHaveClass("invisible");
    expect(field).toHaveAttribute("aria-hidden", "true");
    expect(screen.getByTestId("local-demo-hint").parentElement).toBe(field.parentElement);
    await user.click(screen.getByTestId("local-mode-register"));
    expect(field).not.toHaveClass("invisible");
    expect(screen.getByTestId("local-demo-hint")).toHaveClass("invisible");
  });

  it("no demo hint: Sign in reserves the name field's space at the end of the form, not above Email", async () => {
    useMethods({ founding_open: true });
    renderSignIn();
    const user = userEvent.setup();
    const form = await screen.findByTestId("local-auth-form");
    expect(screen.queryByTestId("local-display-name")).toBeNull();
    const ghost = form.lastElementChild!;
    expect(ghost).toHaveClass("invisible");
    expect(ghost).toHaveAttribute("aria-hidden", "true");
    await user.click(screen.getByTestId("local-mode-register"));
    expect(screen.getByTestId("local-display-name")).toBeVisible();
    expect(form.lastElementChild).not.toHaveClass("invisible");
  });

  // covers: INV-AUTH-09
  it("puts the language control in the card's top-right corner and switches the display language", async () => {
    useMethods({});
    renderSignIn();
    const user = userEvent.setup();
    await screen.findByTestId("signin-google");
    const picker = screen.getByTestId("signin-language-select");
    const card = picker.closest('[data-slot="card"]')!;
    // Corner row: right-aligned, before the logo/header and the form.
    expect(picker.closest(".justify-end")).not.toBeNull();
    expect(card.firstElementChild).toContainElement(picker);
    expect(screen.getByTestId("signin-google")).toHaveTextContent(
      i18n.t("common:signIn.withGoogle", { lng: "en-GB" }),
    );
    await user.selectOptions(picker, "id-ID");
    await waitFor(() =>
      expect(screen.getByTestId("signin-google")).toHaveTextContent(
        i18n.t("common:signIn.withGoogle", { lng: "id-ID" }),
      ),
    );
    await i18n.changeLanguage("en-GB");
  });

  it("renders Continue with Google as the neutral outline button, Sign in stays the primary", async () => {
    useMethods({});
    renderSignIn();
    const google = await screen.findByTestId("signin-google");
    expect(google).toHaveAttribute("data-variant", "outline");
    expect(screen.getByTestId("local-submit")).toHaveAttribute("data-variant", "default");
  });
});
