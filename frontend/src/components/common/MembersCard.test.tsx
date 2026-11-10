// Settings ▸ Membership's members table (#847): every member and each pending,
// unexpired invitation, Revoke as a destructive confirm with no Undo (ADR-0058 §3),
// and Reactivate (ADR-0039) moved into the dormant member's row menu.
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import { toast } from "sonner";
import { server } from "@/test/server";
import { raisedToasts } from "@/test/sonnerMock";
import { renderWithProviders } from "@/test/renderWithProviders";
import { MembersCard } from "@/components/common/MembersCard";
import type { Me } from "@/hooks/useSession";
import type { Membership } from "@/api/types";

vi.mock("sonner", async () => (await import("@/test/sonnerMock")).sonnerMock());
const mockedToast = toast as unknown as Parameters<typeof raisedToasts>[0];

const me: Me = {
  id: "u-alice",
  household_id: "hh-1",
  household_display_name: "Test Household",
  display_name: "Alice",
  nickname: null,
  email: "alice@example.test",
  picture_url: null,
  locale: "en-GB",
  theme: "light",
  time_zone: "UTC",
  reporting_currency: "IDR",
  multi_currency_enabled: false,
  assumed_annual_inflation: "3.5",
  is_founder: true,
};

const membership: Membership = {
  members: [
    {
      id: "u-alice",
      display_name: "Alice",
      nickname: null,
      email: "alice@example.test",
      joined_at: "2026-01-05T12:00:00Z",
      dormant: false,
    },
    {
      id: "u-carol",
      display_name: "Carol",
      nickname: "Caz",
      email: "carol@example.test",
      joined_at: "2026-03-20T12:00:00Z",
      dormant: true,
    },
  ],
  invitations: [
    {
      id: "inv-1",
      invited_email: "dave@example.test",
      sent_at: "2026-11-03T12:00:00Z",
      expires_at: "2026-11-10T12:00:00Z",
    },
  ],
};

const originalWidth = window.innerWidth;

function serve(opts: { me?: Me; local?: boolean; membership?: Membership } = {}) {
  server.use(
    http.get("/api/me", () => HttpResponse.json(opts.me ?? me)),
    http.get("/api/auth/methods", () =>
      HttpResponse.json({
        google: true,
        local: opts.local ?? true,
        password_reset: false,
        demo_mode: false,
      }),
    ),
    http.get("/api/household/membership", () => HttpResponse.json(opts.membership ?? membership)),
  );
}

beforeEach(() => {
  vi.clearAllMocks();
});

afterEach(() => {
  window.innerWidth = originalWidth;
});

// covers: INV-AUTH-31
it("lists every member with a You tag, a joined date and a dormant status, plus each pending invite", async () => {
  serve();
  renderWithProviders(<MembersCard />);

  const alice = await screen.findByTestId("member-row-alice@example.test");
  expect(within(alice).getByText("Alice")).toBeInTheDocument();
  expect(within(alice).getByTestId("member-you-tag")).toHaveTextContent("You");
  expect(within(alice).getByText("5 Jan 2026")).toBeInTheDocument();
  expect(within(alice).queryByTestId("member-dormant-tag")).not.toBeInTheDocument();

  // The nickname wins over the account name, as elsewhere in the app.
  const carol = screen.getByTestId("member-row-carol@example.test");
  expect(within(carol).getByText("Caz")).toBeInTheDocument();
  expect(within(carol).queryByTestId("member-you-tag")).not.toBeInTheDocument();
  expect(within(carol).getByTestId("member-dormant-tag")).toHaveTextContent("Can't sign in yet");
  expect(within(carol).getByText("20 Mar 2026")).toBeInTheDocument();

  const dave = screen.getByTestId("invite-row-dave@example.test");
  expect(within(dave).getByText("3 Nov 2026")).toBeInTheDocument();
  expect(within(dave).getByText("10 Nov 2026")).toBeInTheDocument();
});

it("shows no pending section when nothing is pending", async () => {
  serve({ membership: { ...membership, invitations: [] } });
  renderWithProviders(<MembersCard />);
  await screen.findByTestId("member-table");
  expect(screen.queryByTestId("invite-table")).not.toBeInTheDocument();
  expect(screen.queryByText("Pending invitations")).not.toBeInTheDocument();
});

// covers: INV-AUTH-31
it("Revoke asks first in red, then deletes and raises one plain toast with no Undo", async () => {
  serve();
  let deleted: string | null = null;
  server.use(
    http.delete("/api/invitations/:id", ({ params }) => {
      deleted = params.id as string;
      return new HttpResponse(null, { status: 204 });
    }),
  );
  renderWithProviders(<MembersCard />);

  fireEvent.click(await screen.findByTestId("invite-actions-dave@example.test"));
  fireEvent.click(await screen.findByTestId("revoke-dave@example.test"));

  const dialog = await screen.findByRole("alertdialog");
  expect(within(dialog).getByText("Revoke the invite for dave@example.test?")).toBeInTheDocument();
  expect(deleted, "nothing is sent before the confirm").toBeNull();
  const confirm = within(dialog).getByRole("button", { name: "Revoke invite" });
  expect(confirm).toHaveAttribute("data-variant", "destructive");

  // The refetch after the revoke returns the invite gone.
  server.use(
    http.get("/api/household/membership", () =>
      HttpResponse.json({ ...membership, invitations: [] }),
    ),
  );
  fireEvent.click(confirm);

  await waitFor(() => expect(deleted).toBe("inv-1"));
  await waitFor(() => expect(raisedToasts(mockedToast)).toHaveLength(1));
  const [only] = raisedToasts(mockedToast);
  expect(only.message).toBe("Invite revoked");
  expect(only.options.action, "re-inviting is the recovery, so no Undo").toBeUndefined();
  await waitFor(() =>
    expect(screen.queryByTestId("invite-row-dave@example.test")).not.toBeInTheDocument(),
  );
});

it("cancelling the Revoke confirm sends nothing", async () => {
  serve();
  let deletes = 0;
  server.use(
    http.delete("/api/invitations/:id", () => {
      deletes += 1;
      return new HttpResponse(null, { status: 204 });
    }),
  );
  renderWithProviders(<MembersCard />);

  fireEvent.click(await screen.findByTestId("invite-actions-dave@example.test"));
  fireEvent.click(await screen.findByTestId("revoke-dave@example.test"));
  const dialog = await screen.findByRole("alertdialog");
  fireEvent.click(within(dialog).getByRole("button", { name: "Cancel" }));

  await waitFor(() => expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument());
  expect(deletes).toBe(0);
  expect(raisedToasts(mockedToast)).toHaveLength(0);
});

it("a Revoke the server refuses (already gone) is a red toast, not a success", async () => {
  serve();
  server.use(
    http.delete("/api/invitations/:id", () =>
      HttpResponse.json({ code: "NOT_FOUND" }, { status: 404 }),
    ),
  );
  renderWithProviders(<MembersCard />);

  fireEvent.click(await screen.findByTestId("invite-actions-dave@example.test"));
  fireEvent.click(await screen.findByTestId("revoke-dave@example.test"));
  fireEvent.click(
    within(await screen.findByRole("alertdialog")).getByRole("button", { name: "Revoke invite" }),
  );

  await waitFor(() => expect(mockedToast.error).toHaveBeenCalledTimes(1));
  expect(mockedToast.success).not.toHaveBeenCalled();
});

// covers: INV-AUTH-20
it("Reactivate sits in the dormant member's menu and offers a copy button, never the URL", async () => {
  serve();
  const url = "http://localhost/reset?token=SECRET-TOKEN";
  server.use(
    http.post("/api/auth/local/reactivation", async ({ request }) => {
      expect(await request.json()).toEqual({ user_id: "u-carol" });
      return HttpResponse.json(
        { email: "carol@example.test", set_password_url: url, expires_at: "2026-11-11T12:00:00Z" },
        { status: 201 },
      );
    }),
  );
  const writeText = vi.fn().mockResolvedValue(undefined);
  Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });
  renderWithProviders(<MembersCard />);

  // Only the dormant member has a menu; the active founder row has none.
  await screen.findByTestId("member-row-carol@example.test");
  expect(screen.queryByTestId("member-actions-alice@example.test")).not.toBeInTheDocument();

  fireEvent.click(screen.getByTestId("member-actions-carol@example.test"));
  fireEvent.click(await screen.findByTestId("reactivate-carol@example.test"));

  const dialog = await screen.findByTestId("reactivation-dialog");
  expect(within(dialog).getByText("Sign-in link for carol@example.test")).toBeInTheDocument();
  expect(document.body.textContent).not.toContain("SECRET-TOKEN");

  fireEvent.click(within(dialog).getByTestId("copy-reactivation-link"));
  await waitFor(() => expect(writeText).toHaveBeenCalledWith(url));
  await waitFor(() =>
    expect(within(dialog).getByTestId("copy-reactivation-link")).toHaveTextContent("Copied!"),
  );
  expect(document.body.textContent).not.toContain("SECRET-TOKEN");
});

// covers: INV-AUTH-20
it("offers no Reactivate to a non-founder, or when local auth is off", async () => {
  serve({ me: { ...me, is_founder: false } });
  const { unmount } = renderWithProviders(<MembersCard />);
  await screen.findByTestId("member-row-carol@example.test");
  expect(screen.queryByTestId("member-actions-carol@example.test")).not.toBeInTheDocument();
  unmount();

  serve({ local: false });
  renderWithProviders(<MembersCard />);
  await screen.findByTestId("member-row-carol@example.test");
  expect(screen.queryByTestId("member-actions-carol@example.test")).not.toBeInTheDocument();
});

// covers: INV-PRESENTATION-08
it("stacks into cards at 390px with the same rows and 44px menu triggers", async () => {
  window.innerWidth = 390;
  serve();
  renderWithProviders(<MembersCard />);

  await screen.findByTestId("member-cards");
  expect(screen.queryByTestId("member-table")).not.toBeInTheDocument();
  expect(screen.getByTestId("invite-cards")).toBeInTheDocument();

  const carol = screen.getByTestId("member-row-carol@example.test");
  expect(within(carol).getByText("Joined 20 Mar 2026")).toBeInTheDocument();
  expect(within(carol).getByTestId("member-actions-carol@example.test")).toHaveClass("size-11");

  const dave = screen.getByTestId("invite-row-dave@example.test");
  expect(within(dave).getByText("Sent 3 Nov 2026")).toBeInTheDocument();
  expect(within(dave).getByText("Expires 10 Nov 2026")).toBeInTheDocument();
  expect(within(dave).getByTestId("invite-actions-dave@example.test")).toHaveClass("size-11");
});
