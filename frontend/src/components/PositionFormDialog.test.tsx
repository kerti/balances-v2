// Component test for the shared PositionFormDialog scaffold (ADR-0043 follow-on
// phase, #334). It drives the scaffold with a *synthetic* body — a single
// input, not a real position type — so the test asserts the abstraction
// contract (create-trigger open state, controlled edit mode, the submit→close
// seam, the shared error envelope, footer pending state) rather than any one
// type's fields. Per-type field bodies stay hand-written and are covered by the
// existing @smoke Playwright flows.
import { describe, it, expect, vi } from "vitest";
import { useState } from "react";
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import i18n from "@/i18n";
import { renderWithProviders } from "@/test/renderWithProviders";
import { PositionFormDialog } from "@/components/PositionFormDialog";

const cancelLabel = i18n.t("common:cancel");

describe("PositionFormDialog", () => {
  // covers: INV-PRESENTATION-06
  it("create mode: trigger opens the dialog and renders header + body", async () => {
    const user = userEvent.setup();
    renderWithProviders(
      <PositionFormDialog
        trigger={<button>Add thing</button>}
        title="Create thing"
        description="A new thing"
        submitLabel="Create"
        pendingLabel="Creating…"
        isPending={false}
        error={null}
        onSubmit={() => {}}
      >
        <input aria-label="thing name" />
      </PositionFormDialog>,
    );

    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Add thing" }));

    const dialog = await screen.findByRole("dialog");
    expect(dialog).toHaveTextContent("Create thing");
    expect(dialog).toHaveTextContent("A new thing");
    expect(screen.getByLabelText("thing name")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Create" })).toBeInTheDocument();
  });

  // covers: INV-PRESENTATION-06
  it("submit fires onSubmit with a close callback that dismisses the dialog", async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn<(close: () => void) => void>();
    renderWithProviders(
      <PositionFormDialog
        trigger={<button>Add thing</button>}
        title="Create thing"
        description="A new thing"
        submitLabel="Create"
        pendingLabel="Creating…"
        isPending={false}
        error={null}
        onSubmit={onSubmit}
      >
        <input aria-label="thing name" />
      </PositionFormDialog>,
    );

    await user.click(screen.getByRole("button", { name: "Add thing" }));
    await screen.findByRole("dialog");
    await user.click(screen.getByRole("button", { name: "Create" }));

    expect(onSubmit).toHaveBeenCalledTimes(1);
    // The scaffold hands its own close to the caller; invoking it closes.
    const close = onSubmit.mock.calls[0][0];
    close();
    await waitForDialogGone();
  });

  // covers: INV-PRESENTATION-06
  it("renders the shared error envelope when a mutation error is passed", async () => {
    const user = userEvent.setup();
    renderWithProviders(
      <PositionFormDialog
        trigger={<button>Add thing</button>}
        title="Create thing"
        description="A new thing"
        submitLabel="Create"
        pendingLabel="Creating…"
        isPending={false}
        error={new Error("boom")}
        onSubmit={() => {}}
      >
        <input aria-label="thing name" />
      </PositionFormDialog>,
    );

    await user.click(screen.getByRole("button", { name: "Add thing" }));
    expect(await screen.findByText("boom")).toBeInTheDocument();
  });

  // covers: INV-PRESENTATION-06
  it("footer shows the pending label and disables submit while in flight", async () => {
    const user = userEvent.setup();
    renderWithProviders(
      <PositionFormDialog
        trigger={<button>Add thing</button>}
        title="Create thing"
        description="A new thing"
        submitLabel="Create"
        pendingLabel="Creating…"
        isPending
        error={null}
        onSubmit={() => {}}
      >
        <input aria-label="thing name" />
      </PositionFormDialog>,
    );

    await user.click(screen.getByRole("button", { name: "Add thing" }));
    const submit = await screen.findByRole("button", { name: "Creating…" });
    expect(submit).toBeDisabled();
  });

  // covers: INV-PRESENTATION-06
  it("submitDisabled disables submit but keeps the rest label (not pending)", async () => {
    const user = userEvent.setup();
    renderWithProviders(
      <PositionFormDialog
        trigger={<button>Add thing</button>}
        title="Create thing"
        description="A new thing"
        submitLabel="Create"
        pendingLabel="Creating…"
        isPending={false}
        submitDisabled
        error={null}
        onSubmit={() => {}}
      >
        <input aria-label="thing name" />
      </PositionFormDialog>,
    );

    await user.click(screen.getByRole("button", { name: "Add thing" }));
    // Label stays "Create" (a merely-invalid form must not read as in-flight),
    // but the button is disabled.
    const submit = await screen.findByRole("button", { name: "Create" });
    expect(submit).toBeDisabled();
  });

  // covers: INV-PRESENTATION-06
  it("edit mode: renders open without a trigger and Cancel calls onOpenChange(false)", async () => {
    const user = userEvent.setup();
    function Harness() {
      const [open, setOpen] = useState(true);
      return (
        <PositionFormDialog
          open={open}
          onOpenChange={setOpen}
          title="Edit thing"
          description="Change the thing"
          submitLabel="Save changes"
          pendingLabel="Saving…"
          isPending={false}
          error={null}
          onSubmit={() => {}}
        >
          <input aria-label="thing name" />
        </PositionFormDialog>
      );
    }
    renderWithProviders(<Harness />);

    const dialog = await screen.findByRole("dialog");
    expect(dialog).toHaveTextContent("Edit thing");
    await user.click(screen.getByRole("button", { name: cancelLabel }));
    await waitForDialogGone();
  });
});

async function waitForDialogGone() {
  await vi.waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
}
