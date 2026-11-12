// #840: Create and Edit share one form body per entity, so they show the same
// fields in the same order. Fixed fields (currency, subtype, month) stay in
// their slot, disabled in Edit; only fields the entity cannot take after
// creation (the opening face value / placement date of a bond) are Create-only.
import { describe, expect, it } from "vitest";
import type { ReactElement } from "react";
import { renderWithProviders } from "@/test/renderWithProviders";
import { BankAccountFormBody, type BankAccountFormState } from "./BankAccountForm";
import { PropertyFormBody, type PropertyFormState } from "./PropertyForm";
import { VehicleFormBody, type VehicleFormState } from "./VehicleForm";
import { LiabilityFormBody, type LiabilityFormState } from "./LiabilityForm";
import { ReceivableFormBody, type ReceivableFormState } from "./ReceivableForm";
import { StockFormBody, type StockFormState } from "./StockForm";
import { MutualFundFormBody, type MutualFundFormState } from "./MutualFundForm";
import { GoldFormBody, type GoldFormState } from "./GoldForm";
import { BondFormBody, type BondFormState } from "./BondForm";
import { TimeDepositFormBody, type TimeDepositFormState } from "./TimeDepositForm";
import { SnapshotFormBody, type SnapshotFormState } from "./SnapshotForm";
import {
  QuantityPriceSnapshotFormBody,
  type QuantityPriceSnapshotFormState,
} from "./QuantityPriceSnapshotForm";
import {
  AccruedInterestSnapshotFormBody,
  type AccruedInterestSnapshotFormState,
} from "./AccruedInterestSnapshotForm";
import type { FormMode } from "./mode";

const noop = () => {};

const common = {
  display_name: "",
  description: "",
  ownership_type: "joint",
  entry_type: "acquired",
  sole_owner_user_id: null,
  native_currency: "IDR",
} as const;

// Field labels in DOM order — what a person reads top to bottom.
function labels(ui: ReactElement): string[] {
  const { container } = renderWithProviders(ui);
  return Array.from(container.querySelectorAll("label")).map((l) => l.textContent?.trim() ?? "");
}

function expectSameLayout(
  render: (mode: FormMode) => ReactElement,
  createOnly: string[] = [],
): string[] {
  const create = labels(render("create"));
  const edit = labels(render("edit"));
  expect(edit).toEqual(create.filter((l) => !createOnly.includes(l)));
  return create;
}

describe("position form bodies: Create and Edit render the same field order", () => {
  it("bank account", () => {
    const form: BankAccountFormState = {
      ...common,
      bank_name: "",
      account_number: "",
      account_type: "savings",
    };
    expect(
      expectSameLayout((mode) => (
        <BankAccountFormBody mode={mode} form={form} setForm={noop} soleOwnerID={null} />
      )),
    ).toMatchInlineSnapshot(`
      [
        "Display name",
        "Bank name",
        "Account number",
        "Account type",
        "Currency",
        "Ownership",
        "Joint",
        "Sole owner",
        "Where did this come from?",
        "We funded it with money already tracked here",
        "We already had it, or it came into the household",
        "Description (optional)",
      ]
    `);
  });

  it("property", () => {
    const form: PropertyFormState = {
      ...common,
      property_type: "house",
      address: "",
      acquisition_date: "",
      acquisition_cost: "",
      annual_appreciation_rate: "",
    };
    expect(
      expectSameLayout((mode) => (
        <PropertyFormBody mode={mode} form={form} setForm={noop} soleOwnerID={null} />
      )),
    ).toMatchInlineSnapshot(`
      [
        "Display name",
        "Type",
        "Currency",
        "Address (optional)",
        "Acquisition date (optional)",
        "Acquisition cost (optional)",
        "Annual appreciation rate (% /yr, optional)",
        "Ownership",
        "Joint",
        "Sole owner",
        "Where did this come from?",
        "We funded it with money already tracked here",
        "We already had it, or it came into the household",
        "Description (optional)",
      ]
    `);
  });

  it("vehicle", () => {
    const form: VehicleFormState = {
      ...common,
      vehicle_type: "car",
      make: "",
      model: "",
      year: "",
      plate_number: "",
      annual_depreciation_rate: "",
    };
    expect(
      expectSameLayout((mode) => (
        <VehicleFormBody mode={mode} form={form} setForm={noop} soleOwnerID={null} />
      )),
    ).toMatchInlineSnapshot(`
      [
        "Display name",
        "Type",
        "Currency",
        "Make (optional)",
        "Model (optional)",
        "Year (optional)",
        "Plate number (optional)",
        "Annual depreciation rate (%, optional)",
        "Ownership",
        "Joint",
        "Sole owner",
        "Where did this come from?",
        "We funded it with money already tracked here",
        "We already had it, or it came into the household",
        "Description (optional)",
      ]
    `);
  });

  it("liability", () => {
    const form: LiabilityFormState = {
      ...common,
      subtype: "personal",
      counterparty_name: "",
      principal: "",
      interest_rate: "",
      term_months: "",
      start_date: "",
      maturity_date: "",
    };
    expect(
      expectSameLayout((mode) => (
        <LiabilityFormBody mode={mode} form={form} setForm={noop} soleOwnerID={null} />
      )),
    ).toMatchInlineSnapshot(`
      [
        "Display name",
        "Subtype",
        "Currency",
        "Counterparty",
        "Principal (optional)",
        "Interest rate (% per year, optional)",
        "Term (months, optional)",
        "Start (optional)",
        "Maturity (optional)",
        "Ownership",
        "Joint",
        "Sole owner",
        "Where did this come from?",
        "We took it on while tracking here — what it paid for is recorded too",
        "We already owed it, or it came into the household",
        "Description (optional)",
      ]
    `);
  });

  it("receivable", () => {
    const form: ReceivableFormState = { ...common, counterparty_name: "", due_date: "" };
    expect(
      expectSameLayout((mode) => (
        <ReceivableFormBody mode={mode} form={form} setForm={noop} soleOwnerID={null} />
      )),
    ).toMatchInlineSnapshot(`
      [
        "Display name",
        "Counterparty",
        "Currency",
        "Due date (optional)",
        "Ownership",
        "Joint",
        "Sole owner",
        "Where did this come from?",
        "We funded it with money already tracked here",
        "We already had it, or it came into the household",
        "Description (optional)",
      ]
    `);
  });

  it("stock", () => {
    const form: StockFormState = { ...common, risk_profile: "", ticker: "", exchange: "" };
    expect(
      expectSameLayout((mode) => (
        <StockFormBody mode={mode} form={form} setForm={noop} soleOwnerID={null} />
      )),
    ).toMatchInlineSnapshot(`
      [
        "Display name",
        "Ticker",
        "Exchange",
        "Currency",
        "Ownership",
        "Joint",
        "Sole owner",
        "Where did this come from?",
        "We funded it with money already tracked here",
        "We already had it, or it came into the household",
        "Risk profile",
        "Description (optional)",
      ]
    `);
  });

  it("mutual fund", () => {
    const form: MutualFundFormState = {
      ...common,
      risk_profile: "",
      fund_code: "",
      fund_manager: "",
      fund_type: "",
    };
    expect(
      expectSameLayout((mode) => (
        <MutualFundFormBody mode={mode} form={form} setForm={noop} soleOwnerID={null} />
      )),
    ).toMatchInlineSnapshot(`
      [
        "Display name",
        "Fund code",
        "Fund manager (optional)",
        "Fund type",
        "Currency",
        "Ownership",
        "Joint",
        "Sole owner",
        "Where did this come from?",
        "We funded it with money already tracked here",
        "We already had it, or it came into the household",
        "Description (optional)",
        "Risk profile",
      ]
    `);
  });

  it("gold", () => {
    const form: GoldFormState = { ...common, risk_profile: "", form: "bar", purity: "0.9999" };
    expect(
      expectSameLayout((mode) => (
        <GoldFormBody mode={mode} form={form} setForm={noop} soleOwnerID={null} />
      )),
    ).toMatchInlineSnapshot(`
      [
        "Display name",
        "Form",
        "Purity",
        "Currency",
        "Ownership",
        "Joint",
        "Sole owner",
        "Where did this come from?",
        "We funded it with money already tracked here",
        "We already had it, or it came into the household",
        "Description (optional)",
        "Risk profile",
      ]
    `);
  });

  it("bond (the opening face value and placement date are Create-only)", () => {
    const form: BondFormState = {
      ...common,
      risk_profile: "",
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
    const create = expectSameLayout(
      (mode) => <BondFormBody mode={mode} form={form} setForm={noop} soleOwnerID={null} />,
      ["Face value", "Placement date"],
    );
    expect(create).toMatchInlineSnapshot(`
      [
        "Display name",
        "Series code (optional)",
        "Issuer",
        "Description (optional)",
        "Bought from",
        "Face value",
        "Currency",
        "Placement date",
        "Coupon rate (% per year)",
        "Coupon frequency",
        "Where coupons go",
        "Maturity date",
        "Ownership",
        "Joint",
        "Sole owner",
        "Where did this come from?",
        "We funded it with money already tracked here",
        "We already had it, or it came into the household",
        "Risk profile",
      ]
    `);
  });

  it("time deposit", () => {
    const form: TimeDepositFormState = {
      ...common,
      risk_profile: "",
      bank_name: "",
      principal: "",
      interest_rate: "",
      term_months: "",
      placement_date: "",
      maturity_date: "",
      rollover_policy: "no_rollover",
    };
    expect(
      expectSameLayout((mode) => (
        <TimeDepositFormBody mode={mode} form={form} setForm={noop} soleOwnerID={null} />
      )),
    ).toMatchInlineSnapshot(`
      [
        "Display name",
        "Description (optional)",
        "Bank name",
        "Currency",
        "Principal",
        "Interest rate (% per year)",
        "Term (months)",
        "Placement date",
        "Maturity date",
        "At maturity",
        "Ownership",
        "Joint",
        "Sole owner",
        "Where did this come from?",
        "We funded it with money already tracked here",
        "We already had it, or it came into the household",
        "Risk profile",
      ]
    `);
  });
});

describe("snapshot form bodies: Record and Edit render the same field order", () => {
  const month = { year_month: "2026-05", as_of_date: "", description: "" };

  it("amount-only", () => {
    const form: SnapshotFormState = { ...month, amount: "" };
    expect(
      expectSameLayout((mode) => (
        <SnapshotFormBody mode={mode} currency="IDR" form={form} setForm={noop} />
      )),
    ).toMatchInlineSnapshot(`
      [
        "Month",
        "Statement date (optional)",
        "Amount (IDR)",
        "Note (optional)",
      ]
    `);
  });

  it("quantity + price", () => {
    const form: QuantityPriceSnapshotFormState = { ...month, quantity: "", price_per_unit: "" };
    expect(
      expectSameLayout((mode) => (
        <QuantityPriceSnapshotFormBody
          mode={mode}
          currency="IDR"
          form={form}
          setForm={noop}
          derivedAmount={null}
        />
      )),
    ).toMatchInlineSnapshot(`
      [
        "Month",
        "Statement date (optional)",
        "Quantity",
        "Price per unit (IDR)",
        "Note (optional)",
      ]
    `);
  });

  it("accrued interest", () => {
    const form: AccruedInterestSnapshotFormState = { ...month, amount: "", accrued_interest: "" };
    expect(
      expectSameLayout((mode) => (
        <AccruedInterestSnapshotFormBody
          mode={mode}
          currency="IDR"
          form={form}
          setForm={noop}
          derivedPrincipal={null}
        />
      )),
    ).toMatchInlineSnapshot(`
      [
        "Month",
        "Statement date (optional)",
        "Total value (IDR)",
        "Accrued (IDR)",
        "Note (optional)",
      ]
    `);
  });

  it("accrued interest, time deposit (one Value)", () => {
    const form: AccruedInterestSnapshotFormState = { ...month, amount: "", accrued_interest: "" };
    expect(
      expectSameLayout((mode) => (
        <AccruedInterestSnapshotFormBody
          mode={mode}
          currency="IDR"
          form={form}
          setForm={noop}
          principal="100000000"
        />
      )),
    ).toMatchInlineSnapshot(`
      [
        "Month",
        "Statement date (optional)",
        "Value (IDR)",
        "Note (optional)",
      ]
    `);
  });
});

describe("fixed fields say why they are disabled in Edit", () => {
  const bank: BankAccountFormState = {
    ...common,
    bank_name: "",
    account_number: "",
    account_type: "savings",
  };
  const liability: LiabilityFormState = {
    ...common,
    subtype: "personal",
    counterparty_name: "",
    principal: "",
    interest_rate: "",
    term_months: "",
    start_date: "",
    maturity_date: "",
  };

  it("shows one hint under the currency in Edit and none in Create", () => {
    const create = renderWithProviders(
      <BankAccountFormBody mode="create" form={bank} setForm={noop} soleOwnerID={null} />,
    );
    expect(create.queryByTestId("locked-hint-native_currency")).toBeNull();
    create.unmount();
    const edit = renderWithProviders(
      <BankAccountFormBody mode="edit" form={bank} setForm={noop} soleOwnerID={null} />,
    );
    expect(edit.getByTestId("locked-hint-native_currency")).toHaveTextContent(
      "Can't be changed after creation.",
    );
  });

  it("hints the liability subtype as well as its currency in Edit", () => {
    const { getByTestId } = renderWithProviders(
      <LiabilityFormBody mode="edit" form={liability} setForm={noop} soleOwnerID={null} />,
    );
    expect(getByTestId("locked-hint-subtype")).toBeInTheDocument();
    expect(getByTestId("locked-hint-native_currency")).toBeInTheDocument();
  });
});
