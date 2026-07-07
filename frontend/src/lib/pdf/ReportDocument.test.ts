import { createElement } from "react";
import { describe, it, expect } from "vitest";
import { pdf } from "@react-pdf/renderer";
import type { TFunction } from "i18next";
import { ReportDocument } from "@/lib/pdf/ReportDocument";
import { buildReportPdfData } from "@/lib/pdf/reportPdfData";
import type { MonthlyReport } from "@/api/types";

// Built via React.createElement (not JSX) so this file can stay `.ts` — the
// vitest project split (vitest.config.ts) routes `.ts` to the `node` tier and
// `.tsx` to `jsdom`; react-pdf's rendering is Node-oriented, so we keep it out
// of a jsdom environment entirely rather than assume the interplay is fine.
//
// Deliberately not RTL/DOM-snapshotting <Document>/<Page> — react-pdf uses its
// own reconciler, not react-dom, so render() doesn't apply, and a byte/string
// snapshot would be brittle noise for something meant to be checked visually
// (see the e2e spec + manual verification instead). This is the one
// integration checkpoint: the full pipeline produces a non-empty PDF without
// throwing, for a few representative shapes.
const stubT = ((key: string) => key) as unknown as TFunction;

// pdf()'s type only accepts ReactElement<DocumentProps> — the exact <Document>
// element type, not a wrapper component's own props. JSX call sites satisfy
// this trivially (JSX elements type as ReactElement<any>), but createElement
// preserves ReportDocument's real prop generic, which is a legitimate mismatch
// against react-pdf's narrower type. Scoped cast at this one boundary.
function toPdfElement(el: ReturnType<typeof createElement>): Parameters<typeof pdf>[0] {
  return el as Parameters<typeof pdf>[0];
}

const report = (overrides: Partial<MonthlyReport> = {}): MonthlyReport =>
  ({
    year_month: "2026-06-01T00:00:00Z",
    generated_at: "2026-06-30T00:00:00Z",
    reporting_currency: "IDR",
    nw_total: "100000000",
    nw_assets: "60000000",
    nw_liabilities: "10000000",
    nw_receivables: "5000000",
    nw_investments: "45000000",
    earned_income_total: "8000000",
    investment_return_total: "500000",
    asset_value_change: "0",
    derived_living_expenses: "3000000",
    user_breakdowns: { joint: { nw: "100000000", earned_income: "0", investment_return: "0" } },
    stale_positions: [],
    fx_rates_used: {},
    missing_fx: [],
    ...overrides,
  }) as MonthlyReport;

describe("ReportDocument", () => {
  it("renders a non-baseline month to a non-empty PDF blob", async () => {
    const selected = report();
    const data = buildReportPdfData({
      reports: [selected],
      selected,
      currency: "IDR",
      secondaryCurrency: "",
      rates: [],
    });
    const blob = await pdf(
      toPdfElement(createElement(ReportDocument, { data, t: stubT, members: undefined, me: null })),
    ).toBlob();
    expect(blob.size).toBeGreaterThan(0);
  });

  it("renders a baseline month (suppressed income statement) to a non-empty PDF blob", async () => {
    const selected = report({ derived_living_expenses: null });
    const data = buildReportPdfData({
      reports: [selected],
      selected,
      currency: "IDR",
      secondaryCurrency: "",
      rates: [],
    });
    const blob = await pdf(
      toPdfElement(createElement(ReportDocument, { data, t: stubT, members: undefined, me: null })),
    ).toBlob();
    expect(blob.size).toBeGreaterThan(0);
  });

  it("renders with a secondary display currency to a non-empty PDF blob", async () => {
    const selected = report();
    const data = buildReportPdfData({
      reports: [selected],
      selected,
      currency: "IDR",
      secondaryCurrency: "USD",
      rates: [{ currency: "USD", year_month: "2026-06-01T00:00:00Z", rate: "16000" } as never],
    });
    const blob = await pdf(
      toPdfElement(createElement(ReportDocument, { data, t: stubT, members: undefined, me: null })),
    ).toBlob();
    expect(blob.size).toBeGreaterThan(0);
  });

  it("renders fx rates used and a non-zero asset value change to a non-empty PDF blob", async () => {
    const selected = report({
      fx_rates_used: { USD: "16000" },
      asset_value_change: "-200000",
    });
    const data = buildReportPdfData({
      reports: [selected],
      selected,
      currency: "IDR",
      secondaryCurrency: "",
      rates: [],
    });
    const blob = await pdf(
      toPdfElement(createElement(ReportDocument, { data, t: stubT, members: undefined, me: null })),
    ).toBlob();
    expect(blob.size).toBeGreaterThan(0);
  });

  it("renders a per-member byPerson row, including the viewer's own row, to a non-empty PDF blob", async () => {
    const selected = report({
      user_breakdowns: {
        "user-a": { nw: "60000000", earned_income: "0", investment_return: "0" },
        "user-b": { nw: "40000000", earned_income: "0", investment_return: "0" },
      },
    });
    const data = buildReportPdfData({
      reports: [selected],
      selected,
      currency: "IDR",
      secondaryCurrency: "",
      rates: [],
    });
    const blob = await pdf(
      toPdfElement(
        createElement(ReportDocument, {
          data,
          t: stubT,
          members: [
            { id: "user-a", display_name: "Alice", nickname: null, email: "alice@example.com" },
            { id: "user-b", display_name: "Bob", nickname: null, email: "bob@example.com" },
          ],
          me: { id: "user-a" } as never,
        }),
      ),
    ).toBlob();
    expect(blob.size).toBeGreaterThan(0);
  });
});
