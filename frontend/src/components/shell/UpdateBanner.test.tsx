import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AppUpdateProvider, UpdateBanner } from "@/components/shell/UpdateBanner";

const needRefresh = vi.fn(() => false);
const updateServiceWorker = vi.fn(async () => {});

vi.mock("virtual:pwa-register/react", () => ({
  useRegisterSW: () => ({
    needRefresh: [needRefresh(), () => {}],
    offlineReady: [false, () => {}],
    updateServiceWorker,
  }),
}));

function renderBanner() {
  return render(
    <AppUpdateProvider>
      <UpdateBanner />
    </AppUpdateProvider>,
  );
}

beforeEach(() => {
  needRefresh.mockReturnValue(false);
  updateServiceWorker.mockClear();
});

// covers: INV-PRESENTATION-11
describe("UpdateBanner", () => {
  it("renders nothing while no new build is waiting", () => {
    renderBanner();
    expect(screen.queryByTestId("update-banner")).not.toBeInTheDocument();
  });

  it("offers the reload once a new build is waiting", () => {
    needRefresh.mockReturnValue(true);
    renderBanner();
    expect(screen.getByTestId("update-banner")).toHaveTextContent(
      "A new version of Balances is available.",
    );
    expect(screen.getByRole("button", { name: "Reload" })).toBeInTheDocument();
  });

  it("only swaps to the new build when the user taps Reload", async () => {
    needRefresh.mockReturnValue(true);
    renderBanner();
    expect(updateServiceWorker).not.toHaveBeenCalled();

    await userEvent.click(screen.getByRole("button", { name: "Reload" }));
    expect(updateServiceWorker).toHaveBeenCalledWith(true);
  });
});
