// The Tag field in every position Create and Edit dialog (#839, ADR-0028): the
// Tag is chosen in the dialog and travels in the same write as the position as
// `tag_id` (a uuid assigns, null clears). One toast per save comes from the
// existing position hooks; there is no second tag call.
import { describe, expect, it } from "vitest";
import { http, HttpResponse, type JsonBodyType } from "msw";
import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { server } from "@/test/server";
import { renderWithProviders } from "@/test/renderWithProviders";
import { CreateBankAccountDialog } from "@/components/dialogs/CreateBankAccountDialog";
import { CreateBondDialog } from "@/components/dialogs/CreateBondDialog";
import { CreateGoldDialog } from "@/components/dialogs/CreateGoldDialog";
import { CreateLiabilityDialog } from "@/components/dialogs/CreateLiabilityDialog";
import { CreateMutualFundDialog } from "@/components/dialogs/CreateMutualFundDialog";
import { CreatePropertyDialog } from "@/components/dialogs/CreatePropertyDialog";
import { CreateReceivableDialog } from "@/components/dialogs/CreateReceivableDialog";
import { CreateStockDialog } from "@/components/dialogs/CreateStockDialog";
import { CreateTimeDepositDialog } from "@/components/dialogs/CreateTimeDepositDialog";
import { CreateVehicleDialog } from "@/components/dialogs/CreateVehicleDialog";
import { EditBankAccountDialog } from "@/components/dialogs/EditBankAccountDialog";
import { EditBondDialog } from "@/components/dialogs/EditBondDialog";
import { EditGoldDialog } from "@/components/dialogs/EditGoldDialog";
import { EditLiabilityDialog } from "@/components/dialogs/EditLiabilityDialog";
import { EditMutualFundDialog } from "@/components/dialogs/EditMutualFundDialog";
import { EditPropertyDialog } from "@/components/dialogs/EditPropertyDialog";
import { EditReceivableDialog } from "@/components/dialogs/EditReceivableDialog";
import { EditStockDialog } from "@/components/dialogs/EditStockDialog";
import { EditTimeDepositDialog } from "@/components/dialogs/EditTimeDepositDialog";
import { EditVehicleDialog } from "@/components/dialogs/EditVehicleDialog";

const tags = [
  { id: "tag-goals", name: "Goals", color: "#3b82f6" },
  { id: "tag-retire", name: "Retirement", color: "#10b981" },
];

function stubServer() {
  server.use(
    http.get("/api/tags", () => HttpResponse.json(tags)),
    http.get("/api/me", () =>
      HttpResponse.json({
        id: "u1",
        household_id: "h1",
        household_display_name: "Test Household",
        display_name: "Pat Owner",
        nickname: null,
        email: "pat@example.test",
        picture_url: null,
        locale: "en-GB",
        theme: "system",
        time_zone: "UTC",
        reporting_currency: "IDR",
        multi_currency_enabled: false,
        assumed_annual_inflation: "3.5",
        is_founder: true,
      }),
    ),
    http.get("/api/household/members", () =>
      HttpResponse.json([
        { id: "u1", display_name: "Pat Owner", nickname: null, email: "pat@example.test" },
      ]),
    ),
  );
}

// Records the JSON body of the next write on `path`.
function captureWrite(method: "post" | "patch", path: string, response: JsonBodyType) {
  const box: { body: Record<string, unknown> | null; count: number } = { body: null, count: 0 };
  const handler = method === "post" ? http.post : http.patch;
  server.use(
    handler(path, async ({ request }) => {
      box.body = (await request.json()) as Record<string, unknown>;
      box.count += 1;
      return HttpResponse.json(response, { status: method === "post" ? 201 : 200 });
    }),
  );
  return box;
}

async function pickTag(label: string) {
  const select = await screen.findByTestId("tag-select");
  await screen.findByRole("option", { name: label });
  await userEvent.selectOptions(select, label);
}

const typeInto = (label: string | RegExp, text: string) =>
  userEvent.type(screen.getByLabelText(label), text);
const setValue = (label: string | RegExp, value: string) =>
  fireEvent.change(screen.getByLabelText(label), { target: { value } });

// ----- Create ----------------------------------------------------------------

type CreateCase = {
  name: string;
  path: string;
  dialog: () => React.ReactElement;
  response: JsonBodyType;
  fill: () => Promise<void>;
};

const created = { asset: { id: "n1" }, investment: { id: "n1" }, id: "n1", details: {} };

const createCases: CreateCase[] = [
  {
    name: "bank account",
    path: "/api/bank-accounts",
    dialog: () => <CreateBankAccountDialog />,
    response: created,
    fill: async () => {
      await typeInto("Display name", "Everyday");
      await typeInto("Bank name", "Test Bank");
    },
  },
  {
    name: "property",
    path: "/api/properties",
    dialog: () => <CreatePropertyDialog />,
    response: created,
    fill: async () => {
      await typeInto("Display name", "Family home");
    },
  },
  {
    name: "vehicle",
    path: "/api/vehicles",
    dialog: () => <CreateVehicleDialog />,
    response: created,
    fill: async () => {
      await typeInto("Display name", "Family car");
    },
  },
  {
    name: "liability",
    path: "/api/liabilities",
    dialog: () => <CreateLiabilityDialog />,
    response: created,
    fill: async () => {
      await typeInto("Display name", "Car loan");
      await typeInto("Counterparty", "Test Lender");
    },
  },
  {
    name: "receivable",
    path: "/api/receivables",
    dialog: () => <CreateReceivableDialog />,
    response: created,
    fill: async () => {
      await typeInto("Display name", "Friend loan");
      await typeInto("Counterparty", "Test Friend");
    },
  },
  {
    name: "stock",
    path: "/api/investments/stocks",
    dialog: () => <CreateStockDialog />,
    response: created,
    fill: async () => {
      await typeInto("Display name", "Test stock");
      await typeInto("Ticker", "TEST");
      await typeInto("Exchange", "IDX");
      await userEvent.selectOptions(screen.getByLabelText(/risk profile/i), "low");
    },
  },
  {
    name: "mutual fund",
    path: "/api/investments/mutual-funds",
    dialog: () => <CreateMutualFundDialog />,
    response: created,
    fill: async () => {
      await typeInto("Display name", "Test fund");
      await typeInto("Fund code", "TST-1");
      await userEvent.selectOptions(screen.getByLabelText(/fund type/i), "money_market");
      await userEvent.selectOptions(screen.getByLabelText(/risk profile/i), "low");
    },
  },
  {
    name: "gold",
    path: "/api/investments/golds",
    dialog: () => <CreateGoldDialog />,
    response: created,
    fill: async () => {
      await typeInto("Display name", "Test bar");
      await userEvent.selectOptions(screen.getByLabelText(/risk profile/i), "low");
    },
  },
  {
    name: "bond",
    path: "/api/investments/bonds",
    dialog: () => <CreateBondDialog />,
    response: created,
    fill: async () => {
      await typeInto("Display name", "Test bond");
      await typeInto("Issuer", "Test Issuer");
      await typeInto("Face value", "10000000");
      setValue("Placement date", "2026-01-15");
      await typeInto(/Coupon rate/, "6.25");
      setValue("Maturity date", "2030-01-15");
      await userEvent.selectOptions(screen.getByLabelText(/risk profile/i), "low");
    },
  },
  {
    name: "time deposit",
    path: "/api/investments/time-deposits",
    dialog: () => <CreateTimeDepositDialog />,
    response: created,
    fill: async () => {
      await typeInto("Display name", "Test deposit");
      await typeInto("Bank name", "Test Bank");
      await typeInto("Principal", "50000000");
      await typeInto(/Interest rate/, "5.5");
      setValue("Placement date", "2026-01-15");
      await typeInto(/Term/, "12");
      setValue("Maturity date", "2027-01-15");
      await userEvent.selectOptions(screen.getByLabelText(/risk profile/i), "low");
    },
  },
];

describe("Create dialogs carry the Tag", () => {
  // covers: INV-TAGS-07
  it.each(createCases)("$name: the chosen Tag is sent as tag_id", async (c) => {
    stubServer();
    const write = captureWrite("post", c.path, c.response);
    renderWithProviders(c.dialog());
    await userEvent.click(screen.getByRole("button"));
    await screen.findByRole("dialog");
    await c.fill();
    await pickTag("Retirement");
    await userEvent.click(screen.getByRole("button", { name: "Create" }));

    await waitFor(() => expect(write.count).toBe(1));
    expect(write.body?.tag_id).toBe("tag-retire");
  });

  // covers: INV-TAGS-07
  it.each(createCases)("$name: no Tag chosen sends tag_id null", async (c) => {
    stubServer();
    const write = captureWrite("post", c.path, c.response);
    renderWithProviders(c.dialog());
    await userEvent.click(screen.getByRole("button"));
    await screen.findByRole("dialog");
    await c.fill();
    await userEvent.click(screen.getByRole("button", { name: "Create" }));

    await waitFor(() => expect(write.count).toBe(1));
    expect(write.body).toHaveProperty("tag_id", null);
  });
});

// ----- Edit ------------------------------------------------------------------

function core(tagId: string | null, subtype: string, extra: Record<string, unknown> = {}) {
  return {
    id: "p1",
    household_id: "h1",
    display_name: "Existing",
    description: null,
    subtype,
    ownership_type: "joint",
    sole_owner_user_id: null,
    native_currency: "IDR",
    status: "active",
    terminated_at: null,
    termination_note: null,
    tag_id: tagId,
    entry_type: "acquired",
    risk_profile: "medium",
    ...extra,
  };
}

type EditCase = {
  name: string;
  path: string;
  dialog: (tagId: string | null) => React.ReactElement;
};

const noop = () => {};
const edit = (el: unknown) => el as never;

const editCases: EditCase[] = [
  {
    name: "bank account",
    path: "/api/bank-accounts/p1",
    dialog: (tag) => (
      <EditBankAccountDialog
        open
        onOpenChange={noop}
        account={edit({
          asset: core(tag, "bank_account"),
          details: { bank_name: "BCA", account_number: "1", account_type: "savings" },
        })}
      />
    ),
  },
  {
    name: "property",
    path: "/api/properties/p1",
    dialog: (tag) => (
      <EditPropertyDialog
        open
        onOpenChange={noop}
        property={edit({
          asset: core(tag, "property"),
          details: {
            property_type: "house",
            address: null,
            acquisition_date: null,
            acquisition_cost: null,
            annual_appreciation_rate: null,
          },
        })}
      />
    ),
  },
  {
    name: "vehicle",
    path: "/api/vehicles/p1",
    dialog: (tag) => (
      <EditVehicleDialog
        open
        onOpenChange={noop}
        vehicle={edit({
          asset: core(tag, "vehicle"),
          details: {
            vehicle_type: "car",
            make: null,
            model: null,
            year: null,
            plate_number: null,
            annual_depreciation_rate: null,
          },
        })}
      />
    ),
  },
  {
    name: "liability",
    path: "/api/liabilities/p1",
    dialog: (tag) => (
      <EditLiabilityDialog
        open
        onOpenChange={noop}
        liability={edit(
          core(tag, "personal", {
            counterparty_name: "Uncle",
            principal: null,
            interest_rate: null,
            term_months: null,
            start_date: null,
            maturity_date: null,
          }),
        )}
      />
    ),
  },
  {
    name: "receivable",
    path: "/api/receivables/p1",
    dialog: (tag) => (
      <EditReceivableDialog
        open
        onOpenChange={noop}
        receivable={edit(core(tag, "receivable", { counterparty_name: "Brother", due_date: null }))}
      />
    ),
  },
  {
    name: "stock",
    path: "/api/investments/stocks/p1",
    dialog: (tag) => (
      <EditStockDialog
        open
        onOpenChange={noop}
        stock={edit({
          investment: core(tag, "stock"),
          details: { ticker: "TEST", exchange: "IDX" },
        })}
      />
    ),
  },
  {
    name: "mutual fund",
    path: "/api/investments/mutual-funds/p1",
    dialog: (tag) => (
      <EditMutualFundDialog
        open
        onOpenChange={noop}
        mutualFund={edit({
          investment: core(tag, "mutual_fund"),
          details: { fund_code: "TST-1", fund_manager: null, fund_type: "money_market" },
        })}
      />
    ),
  },
  {
    name: "gold",
    path: "/api/investments/golds/p1",
    dialog: (tag) => (
      <EditGoldDialog
        open
        onOpenChange={noop}
        gold={edit({
          investment: core(tag, "gold"),
          details: { form: "bar", purity: "0.9999" },
        })}
      />
    ),
  },
  {
    name: "bond",
    path: "/api/investments/bonds/p1",
    dialog: (tag) => (
      <EditBondDialog
        open
        onOpenChange={noop}
        bond={edit({
          investment: core(tag, "bond"),
          details: {
            bond_type: "secondary_market",
            series_code: null,
            issuer: "Test Issuer",
            coupon_rate: "6.25",
            coupon_frequency: "monthly",
            coupon_disposition: "pays_out",
            maturity_date: "2030-01-15T00:00:00Z",
          },
        })}
      />
    ),
  },
  {
    name: "time deposit",
    path: "/api/investments/time-deposits/p1",
    dialog: (tag) => (
      <EditTimeDepositDialog
        open
        onOpenChange={noop}
        timeDeposit={edit({
          investment: core(tag, "time_deposit"),
          details: {
            bank_name: "Test Bank",
            principal: "50000000",
            interest_rate: "5.5",
            term_months: 12,
            placement_date: "2026-01-15T00:00:00Z",
            maturity_date: "2027-01-15T00:00:00Z",
            rollover_policy: "no_rollover",
          },
        })}
      />
    ),
  },
];

describe("Edit dialogs seed, change and clear the Tag", () => {
  // covers: INV-TAGS-07
  it.each(editCases)(
    "$name: preselects the saved Tag; change and clear are sent as tag_id",
    async (c) => {
      stubServer();
      const write = captureWrite("patch", c.path, {});
      renderWithProviders(c.dialog("tag-goals"));
      const dialog = await screen.findByRole("dialog");

      // Seeded from the position's own tag_id.
      const select = await within(dialog).findByTestId("tag-select");
      await within(dialog).findByRole("option", { name: "Retirement" });
      await waitFor(() => expect(select).toHaveValue("tag-goals"));

      // Change it.
      await userEvent.selectOptions(select, "Retirement");
      await userEvent.click(within(dialog).getByRole("button", { name: "Save changes" }));
      await waitFor(() => expect(write.count).toBe(1));
      expect(write.body?.tag_id).toBe("tag-retire");

      // Clear it: null, not absent, so the server clears rather than leaves it.
      await userEvent.selectOptions(select, "No tag");
      await userEvent.click(within(dialog).getByRole("button", { name: "Save changes" }));
      await waitFor(() => expect(write.count).toBe(2));
      expect(write.body).toHaveProperty("tag_id", null);
    },
  );

  // covers: INV-TAGS-07
  it.each(editCases)(
    "$name: an untagged position opens on No tag and saves tag_id null",
    async (c) => {
      stubServer();
      const write = captureWrite("patch", c.path, {});
      renderWithProviders(c.dialog(null));
      const dialog = await screen.findByRole("dialog");

      const select = await within(dialog).findByTestId("tag-select");
      expect(select).toHaveValue("");
      await userEvent.click(within(dialog).getByRole("button", { name: "Save changes" }));
      await waitFor(() => expect(write.count).toBe(1));
      expect(write.body).toHaveProperty("tag_id", null);
    },
  );
});
