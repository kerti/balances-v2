// The client mirror of a bond's term window (#574, INV-BONDS-05): the server owns
// the rule; the date inputs bound what they can so a household member is stopped
// in the form, and the server's two term codes render as readable copy.
import { describe, expect, it, vi } from "vitest";
import { fireEvent, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import i18n from "@/i18n";
import { ApiError } from "@/api/client";
import { renderWithProviders } from "@/test/renderWithProviders";
import { BondFormBody, type BondFormState } from "@/components/dialogs/forms/BondForm";
import { CreateBondDialog } from "@/components/dialogs/CreateBondDialog";
import { todayDate } from "@/lib/dateLimits";
import enErrors from "@/locales/en/errors.json";
import idErrors from "@/locales/id/errors.json";

let mutationError: unknown = null;
vi.mock("@/hooks/useSession", () => ({
  useSession: () => ({ data: { id: "u1", reporting_currency: "IDR" } }),
}));
vi.mock("@/hooks/useInvestments", () => ({
  useCreateBond: () => ({
    mutate: mutateSpy,
    reset: vi.fn(),
    isPending: false,
    isError: mutationError != null,
    error: mutationError,
  }),
}));
const mutateSpy = vi.fn();

const base: BondFormState = {
  display_name: "",
  description: "",
  ownership_type: "joint",
  entry_type: "acquired",
  sole_owner_user_id: null,
  risk_profile: "",
  native_currency: "IDR",
  bond_type: "govt_primary",
  series_code: "",
  issuer: "",
  face_value: "",
  placement_date: "",
  coupon_rate: "",
  coupon_frequency: "monthly",
  coupon_disposition: "pays_out",
  maturity_date: "",
};

const noop = () => {};
const placement = () => screen.getByLabelText(/placement date/i) as HTMLInputElement;
const maturity = () => screen.getByLabelText(/maturity date/i) as HTMLInputElement;

// covers: INV-BONDS-05
describe("bond term window: date inputs", () => {
  it("Create caps the placement at today and floors the maturity the day after it", () => {
    renderWithProviders(
      <BondFormBody
        mode="create"
        form={{ ...base, placement_date: "2026-01-31" }}
        setForm={noop}
        soleOwnerID={null}
      />,
    );
    expect(placement()).toHaveAttribute("max", todayDate());
    expect(maturity()).toHaveAttribute("min", "2026-02-01");
  });

  it("Create gives maturity no floor until a placement is typed", () => {
    renderWithProviders(
      <BondFormBody mode="create" form={base} setForm={noop} soleOwnerID={null} />,
    );
    expect(maturity()).not.toHaveAttribute("min");
  });

  it("Create of a secondary-market bond has no placement and no floor (no Buy, no term start)", () => {
    renderWithProviders(
      <BondFormBody
        mode="create"
        form={{ ...base, bond_type: "secondary_market", placement_date: "2026-01-31" }}
        setForm={noop}
        soleOwnerID={null}
      />,
    );
    expect(screen.queryByLabelText(/placement date/i)).not.toBeInTheDocument();
    expect(maturity()).not.toHaveAttribute("min");
  });

  it("Edit floors the maturity the day after the earliest Buy", () => {
    renderWithProviders(
      <BondFormBody
        mode="edit"
        form={{ ...base, maturity_date: "2030-01-01" }}
        setForm={noop}
        soleOwnerID={null}
        firstBuyDate="2026-12-31"
      />,
    );
    expect(maturity()).toHaveAttribute("min", "2027-01-01");
  });

  it("Edit has no floor while there is no Buy, and never shows the placement input", () => {
    renderWithProviders(
      <BondFormBody
        mode="edit"
        form={{ ...base, maturity_date: "2030-01-01" }}
        setForm={noop}
        soleOwnerID={null}
      />,
    );
    expect(maturity()).not.toHaveAttribute("min");
    expect(screen.queryByLabelText(/placement date/i)).not.toBeInTheDocument();
  });
});

// covers: INV-BONDS-05
describe("bond term window: Create dialog", () => {
  async function open() {
    mutateSpy.mockClear();
    renderWithProviders(<CreateBondDialog />);
    await userEvent.click(screen.getByRole("button", { name: /new bond/i }));
    return screen.findByRole("button", { name: i18n.t("common:actions.create") });
  }

  it("refuses a future placement in the form, without calling the server", async () => {
    mutationError = null;
    const create = await open();
    fireEvent.change(placement(), { target: { value: "2999-01-01" } });
    await userEvent.click(create);

    expect(mutateSpy).not.toHaveBeenCalled();
    expect(screen.getByTestId("field-error-bond_placement_date")).toHaveTextContent(
      /on or before/i,
    );
  });

  it("refuses a maturity on or before the placement in the form", async () => {
    mutationError = null;
    const create = await open();
    fireEvent.change(placement(), { target: { value: "2026-01-15" } });
    fireEvent.change(maturity(), { target: { value: "2026-01-15" } });
    await userEvent.click(create);

    expect(mutateSpy).not.toHaveBeenCalled();
    expect(screen.getByTestId("field-error-bond_maturity")).toHaveTextContent(
      /on or after .*2026/i,
    );
  });

  it("renders INVALID_BOND_TERM beside the maturity field in the household's language", async () => {
    mutationError = new ApiError(400, "bad request", { code: "INVALID_BOND_TERM" });
    await open();
    expect(await screen.findByTestId("field-error-bond_maturity")).toHaveTextContent(
      i18n.t("errors:code.INVALID_BOND_TERM"),
    );
  });

  it("renders OUTSIDE_BOND_TERM as the form message, not a raw code", async () => {
    mutationError = new ApiError(400, "bad request", { code: "OUTSIDE_BOND_TERM" });
    await open();
    expect(await screen.findByTestId("form-error")).toHaveTextContent(
      i18n.t("errors:code.OUTSIDE_BOND_TERM"),
    );
    expect(i18n.t("errors:code.OUTSIDE_BOND_TERM")).not.toContain("OUTSIDE_BOND_TERM");
  });

  it("has Indonesian copy for both codes, distinct from the English", () => {
    for (const code of ["INVALID_BOND_TERM", "OUTSIDE_BOND_TERM"] as const) {
      const idText = idErrors.code[code];
      expect(idText).toBeTruthy();
      expect(idText).not.toBe(enErrors.code[code]);
      expect(idText).not.toContain(code);
    }
  });
});
