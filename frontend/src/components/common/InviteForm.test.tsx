// The invite form's result state (#847): "Invite sent to x@y" plus a Copy invite
// link button. The tokenised accept URL is a bearer secret and is never rendered.
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { fireEvent, screen, waitFor } from "@testing-library/react";
import { toast } from "sonner";
import { server } from "@/test/server";
import { raisedToasts } from "@/test/sonnerMock";
import { renderWithProviders } from "@/test/renderWithProviders";
import { InviteForm } from "@/components/common/InviteForm";

vi.mock("sonner", async () => (await import("@/test/sonnerMock")).sonnerMock());
const mockedToast = toast as unknown as Parameters<typeof raisedToasts>[0];

const ACCEPT_URL = "http://localhost:5173/accept?token=SECRET-TOKEN&lng=en-GB";

const originalClipboard = Object.getOwnPropertyDescriptor(navigator, "clipboard");

function mockClipboard(writeText: () => Promise<void>) {
  Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });
}

function serveInvite(emailSent = true) {
  server.use(
    http.post("/api/invitations", async ({ request }) => {
      const body = (await request.json()) as { email: string };
      return HttpResponse.json(
        {
          id: "inv-1",
          invited_email: body.email,
          expires_at: "2026-11-17T12:00:00Z",
          accept_url: ACCEPT_URL,
          email_sent: emailSent,
        },
        { status: 201 },
      );
    }),
    http.get("/api/household/membership", () =>
      HttpResponse.json({ members: [], invitations: [] }),
    ),
  );
}

async function send(email: string) {
  fireEvent.change(screen.getByLabelText("Email address"), { target: { value: email } });
  fireEvent.click(screen.getByRole("button", { name: "Send invitation" }));
}

beforeEach(() => {
  vi.clearAllMocks();
});

afterEach(() => {
  if (originalClipboard) Object.defineProperty(navigator, "clipboard", originalClipboard);
});

// covers: INV-AUTH-32
it("says who the invite went to and copies the link without ever printing it", async () => {
  serveInvite();
  const writeText = vi.fn().mockResolvedValue(undefined);
  mockClipboard(writeText);
  renderWithProviders(<InviteForm />);

  await send("dave@example.test");

  expect(await screen.findByText("Invite sent to dave@example.test")).toBeInTheDocument();
  const copy = screen.getByTestId("copy-invite-link");
  expect(copy).toHaveTextContent("Copy invite link");
  // No raw URL (or any trace of the token) anywhere in the DOM.
  expect(document.body.textContent).not.toContain("SECRET-TOKEN");
  expect(document.body.textContent).not.toContain("/accept?");
  expect(screen.queryByTestId("invite-accept-url")).not.toBeInTheDocument();

  fireEvent.click(copy);
  await waitFor(() => expect(writeText).toHaveBeenCalledWith(ACCEPT_URL));
  await waitFor(() => expect(copy).toHaveTextContent("Copied!"));
  expect(document.body.textContent).not.toContain("SECRET-TOKEN");
});

// covers: INV-NOTIFICATIONS-11
it("still nudges with a toast when the email could not be sent", async () => {
  serveInvite(false);
  mockClipboard(vi.fn().mockResolvedValue(undefined));
  renderWithProviders(<InviteForm />);

  await send("dave@example.test");

  await screen.findByTestId("copy-invite-link");
  expect(mockedToast.warning).toHaveBeenCalledTimes(1);
});

it("a denied clipboard raises one error toast and leaves the button usable", async () => {
  serveInvite();
  mockClipboard(vi.fn().mockRejectedValue(new Error("denied")));
  renderWithProviders(<InviteForm />);

  await send("dave@example.test");
  const copy = await screen.findByTestId("copy-invite-link");
  fireEvent.click(copy);

  await waitFor(() => expect(mockedToast.error).toHaveBeenCalledTimes(1));
  expect(copy).toHaveTextContent("Copy invite link");
  expect(copy).toBeEnabled();
  expect(document.body.textContent).not.toContain("SECRET-TOKEN");
});

// covers: INV-PRESENTATION-19
it("never disables Send for an empty address: it says what is missing and sends nothing", async () => {
  let posts = 0;
  server.use(
    http.post("/api/invitations", () => {
      posts += 1;
      return HttpResponse.json({}, { status: 201 });
    }),
  );
  renderWithProviders(<InviteForm />);

  const submit = screen.getByRole("button", { name: "Send invitation" });
  expect(submit).toBeEnabled();
  fireEvent.click(submit);

  expect(await screen.findByTestId("field-error-email")).toBeInTheDocument();
  expect(posts).toBe(0);
});

it("answers inviting yourself under the email field", async () => {
  server.use(
    http.post("/api/invitations", () =>
      HttpResponse.json({ code: "CANNOT_INVITE_SELF" }, { status: 400 }),
    ),
  );
  renderWithProviders(<InviteForm />);

  await send("me@example.test");

  expect(await screen.findByTestId("field-error-email")).toHaveTextContent(
    "You can't invite yourself.",
  );
});
