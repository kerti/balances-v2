import { test, expect, type Page } from "@playwright/test";
import { expectNoControlOverlap } from "./overlap";

// Measured guard: no two interactive controls overlap on the main screens and
// the dense dialogs, at phone (390px) and tablet (900px) widths. #865 shipped a
// month picker drawn 85px over the statement-date field at 390px because nobody
// measured; `expectNoControlOverlap` does (INV-PRESENTATION-26).
//
// One test, fixtures seeded over the API and removed in `finally`, so it needs
// nothing from other specs. Soft assertions: a failing run lists every
// overlapping pair across all screens and both widths, not just the first.
const VIEWPORTS = [
  { width: 390, height: 844 },
  { width: 900, height: 800 },
];

function monthsAgo(n: number) {
  const d = new Date();
  d.setUTCDate(1);
  d.setUTCMonth(d.getUTCMonth() - n);
  return `${d.getUTCFullYear()}-${String(d.getUTCMonth() + 1).padStart(2, "0")}`;
}

async function created(res: { ok(): boolean; json(): Promise<unknown> }, what: string) {
  expect(res.ok(), `seed ${what}`).toBeTruthy();
  return res.json();
}

async function openAndCheck(page: Page, open: () => Promise<void>, label: string) {
  await open();
  const dialog = page.getByRole("dialog");
  await expect(dialog).toBeVisible();
  await expectNoControlOverlap(page, dialog, label);
  await page.keyboard.press("Escape");
  await expect(dialog).toHaveCount(0);
}

// covers: INV-PRESENTATION-26
test(
  "no two interactive controls overlap on the main screens and dialogs at 390 and 900px",
  { tag: "@smoke" },
  async ({ page }) => {
    test.setTimeout(240_000);
    const stamp = Date.now();
    const accountName = `E2E overlap account ${stamp}`;
    const stockName = `E2E overlap stock ${stamp}`;
    const depositName = `E2E overlap deposit ${stamp}`;
    const bondName = `E2E overlap bond ${stamp}`;
    const tagName = `E2E overlap tag ${stamp}`;
    const ids: {
      account?: string;
      stock?: string;
      deposit?: string;
      bond?: string;
      tag?: string;
      income?: string;
    } = {};

    try {
      const account = (await created(
        await page.request.post("/api/bank-accounts", {
          data: {
            display_name: accountName,
            description: null,
            ownership_type: "joint",
            sole_owner_user_id: null,
            native_currency: "IDR",
            bank_name: "E2E Bank",
            account_number: "",
            account_type: "savings",
            entry_type: "newly_tracked",
          },
        }),
        "bank account",
      )) as { asset: { id: string } };
      ids.account = account.asset.id;
      for (const [ym, amount] of [
        [monthsAgo(1), "1500000"],
        [monthsAgo(0), "1750000"],
      ]) {
        await created(
          await page.request.post(`/api/assets/${ids.account}/snapshots`, {
            data: { year_month: ym, amount, currency: "IDR", as_of_date: null, description: null },
          }),
          "account snapshot",
        );
      }

      const stock = (await created(
        await page.request.post("/api/investments/stocks", {
          data: {
            display_name: stockName,
            description: null,
            ownership_type: "joint",
            sole_owner_user_id: null,
            risk_profile: "medium",
            native_currency: "IDR",
            ticker: "OVL",
            exchange: "TEST",
            entry_type: "newly_tracked",
          },
        }),
        "stock",
      )) as { investment: { id: string } };
      ids.stock = stock.investment.id;
      await created(
        await page.request.post(`/api/investments/${ids.stock}/snapshots`, {
          data: {
            year_month: monthsAgo(1),
            amount: "850000",
            currency: "IDR",
            quantity: "100",
            price_per_unit: "8500",
            accrued_interest: null,
            as_of_date: null,
            description: null,
          },
        }),
        "stock snapshot",
      );

      const placed = `${monthsAgo(2)}-10`;
      const matures = `${monthsAgo(-10)}-10`;
      const deposit = (await created(
        await page.request.post("/api/investments/time-deposits", {
          data: {
            display_name: depositName,
            description: null,
            ownership_type: "joint",
            sole_owner_user_id: null,
            risk_profile: "low",
            native_currency: "IDR",
            bank_name: "E2E Bank",
            principal: "1000000",
            interest_rate: "5",
            term_months: 12,
            placement_date: placed,
            maturity_date: matures,
            rollover_policy: "no_rollover",
            entry_type: "newly_tracked",
          },
        }),
        "time deposit",
      )) as { investment: { id: string } };
      ids.deposit = deposit.investment.id;
      await created(
        await page.request.post(`/api/investments/${ids.deposit}/snapshots`, {
          data: {
            year_month: monthsAgo(1),
            amount: "1008000",
            currency: "IDR",
            quantity: null,
            price_per_unit: null,
            accrued_interest: "8000",
            as_of_date: null,
            description: null,
          },
        }),
        "deposit snapshot",
      );

      const bond = (await created(
        await page.request.post("/api/investments/bonds", {
          data: {
            display_name: bondName,
            description: null,
            ownership_type: "joint",
            sole_owner_user_id: null,
            risk_profile: "medium",
            native_currency: "IDR",
            bond_type: "govt_primary",
            series_code: "OVLB",
            issuer: "E2E Treasury",
            face_value: "1000000",
            placement_date: placed,
            coupon_rate: "6.5",
            coupon_frequency: "semi_annual",
            coupon_disposition: "pays_out",
            maturity_date: matures,
            entry_type: "newly_tracked",
          },
        }),
        "bond",
      )) as { investment: { id: string } };
      ids.bond = bond.investment.id;

      const tag = (await created(
        await page.request.post("/api/tags", { data: { name: tagName, color: "#3b82f6" } }),
        "tag",
      )) as { id: string };
      ids.tag = tag.id;
      const assigned = await page.request.put("/api/tags/assignments", {
        data: { tag_id: ids.tag, items: [{ group: "asset", position_id: ids.account }] },
      });
      expect(assigned.ok(), "seed tag assignment").toBeTruthy();

      const income = (await created(
        await page.request.post("/api/income", {
          data: {
            date: `${monthsAgo(0)}-01`,
            amount: "5000000",
            currency: "IDR",
            category: "salary",
            description: "E2E overlap income",
            ownership_type: "joint",
            sole_owner_user_id: null,
            regularity: "routine",
          },
        }),
        "income",
      )) as { id: string };
      ids.income = income.id;

      for (const viewport of VIEWPORTS) {
        await page.setViewportSize(viewport);
        const at = (what: string) => `${what} @${viewport.width}px`;

        // --- Screens ---
        await page.goto("/");
        await expect(page.getByRole("heading", { level: 1, name: "Net worth" })).toBeVisible();
        await expectNoControlOverlap(page, undefined, at("dashboard"));

        await page.goto("/assets/bank-accounts");
        await expect(page.getByText(accountName).first()).toBeVisible();
        await expectNoControlOverlap(page, undefined, at("bank-account list"));

        await page.goto(`/assets/bank-accounts/${ids.account}`);
        await expect(page.getByRole("heading", { level: 1, name: accountName })).toBeVisible();
        await expect(page.getByTestId("snapshot-row").first()).toBeVisible();
        await expectNoControlOverlap(page, undefined, at("bank-account detail"));

        await page.goto("/investments/enter/prices");
        await expect(page.getByText(stockName).first()).toBeVisible();
        await expectNoControlOverlap(page, undefined, at("Enter prices"));

        await page.goto("/investments/enter/accrued");
        await expect(page.getByText(depositName).first()).toBeVisible();
        await expectNoControlOverlap(page, undefined, at("bulk bonds & deposits entry"));

        await page.goto("/income");
        await expect(page.getByTestId("income-row").first()).toBeVisible();
        await expectNoControlOverlap(page, undefined, at("Income"));

        await page.goto("/tags");
        await expect(page.getByTestId("tag-breakdown-IDR")).toBeVisible();
        await expectNoControlOverlap(page, undefined, at("Tags"));

        // --- The open drawer (both widths are below the 1024px shell boundary) ---
        await page.goto("/");
        await page.getByRole("button", { name: "Toggle Sidebar" }).click();
        const drawer = page.getByRole("dialog");
        await expect(drawer).toBeVisible();
        await expectNoControlOverlap(page, drawer, at("drawer"));
        await page.getByTestId("drawer-close").click();
        await expect(drawer).toHaveCount(0);

        // --- Dialogs ---
        await page.goto(`/assets/bank-accounts/${ids.account}`);
        await expect(page.getByRole("heading", { level: 1, name: accountName })).toBeVisible();
        await openAndCheck(
          page,
          () => page.getByRole("button", { name: "Record snapshot" }).click(),
          at("Record snapshot (amount)"),
        );
        await openAndCheck(
          page,
          () => page.getByRole("button", { name: "Edit", exact: true }).click(),
          at("Edit bank account"),
        );

        await page.goto(`/investments/stocks/${ids.stock}`);
        await expect(page.getByRole("heading", { level: 1, name: stockName })).toBeVisible();
        await openAndCheck(
          page,
          () => page.getByRole("button", { name: "Record snapshot" }).click(),
          at("Record snapshot (quantity x price)"),
        );
        await page.goto(`/investments/stocks/${ids.stock}?tab=transactions`);
        await expect(page.getByRole("heading", { level: 1, name: stockName })).toBeVisible();
        await openAndCheck(
          page,
          () => page.getByRole("button", { name: "Buy" }).click(),
          at("Buy"),
        );

        await page.goto(`/investments/time-deposits/${ids.deposit}`);
        await expect(page.getByRole("heading", { level: 1, name: depositName })).toBeVisible();
        await openAndCheck(
          page,
          () => page.getByRole("button", { name: "Record snapshot" }).click(),
          at("Record snapshot (accrued interest)"),
        );

        await page.goto("/income");
        await expect(page.getByTestId("income-row").first()).toBeVisible();
        await openAndCheck(
          page,
          () => page.getByRole("button", { name: /^New( income)?$/ }).click(),
          at("New income"),
        );
      }

      // --- The bond detail page (#839): the header (? · Edit · ⋯), the details
      // card's identity line + key/value grid, and the Snapshots | Transactions
      // tab bar, at tablet width (768px, where the grid goes from 3 columns to 2
      // inside a card narrowed by the docked sidebar) and phone width. ---
      for (const width of [768, 390]) {
        await page.setViewportSize({ width, height: 900 });
        const at = (what: string) => `${what} @${width}px`;
        await page.goto(`/investments/bonds/${ids.bond}`);
        await expect(page.getByRole("heading", { level: 1, name: bondName })).toBeVisible();
        await expect(page.getByTestId("history-tab-transactions")).toBeVisible();
        await expectNoControlOverlap(
          page,
          page.locator('[data-slot="page-header"]'),
          at("bond detail header"),
        );
        await expectNoControlOverlap(
          page,
          page.getByTestId("tour-details"),
          at("bond detail details card"),
        );
        await expectNoControlOverlap(page, undefined, at("bond detail"));

        await page.getByTestId("position-menu-trigger").click();
        const menu = page.getByRole("menu");
        await expect(menu).toBeVisible();
        await expectNoControlOverlap(page, menu, at("bond detail header menu"));
        await page.keyboard.press("Escape");

        await page.getByTestId("history-tab-transactions").click();
        await expect(page).toHaveURL(/[?&]tab=transactions/);
        await expectNoControlOverlap(page, undefined, at("bond detail, Transactions tab"));
      }

      // --- Range chips and Brush (#901). Chips only exist for a history over
      // 12 months, so give the account 15 months and check the chart screens
      // that carry them: the position detail, the Assets overview and the
      // dashboard. ---
      for (let n = 2; n <= 14; n++) {
        await created(
          await page.request.post(`/api/assets/${ids.account}/snapshots`, {
            data: {
              year_month: monthsAgo(n),
              amount: String(1_000_000 + n * 10_000),
              currency: "IDR",
              as_of_date: null,
              description: null,
            },
          }),
          "account history snapshot",
        );
      }
      for (const width of [768, 390]) {
        await page.setViewportSize({ width, height: 900 });
        const at = (what: string) => `${what} @${width}px`;
        await page.goto(`/assets/bank-accounts/${ids.account}`);
        await expect(page.getByRole("heading", { level: 1, name: accountName })).toBeVisible();
        await expect(page.getByTestId("chart-range-chips")).toBeVisible();
        await expectNoControlOverlap(page, undefined, at("bank-account detail with range chips"));
        await page.goto("/assets");
        await expect(page.getByRole("heading", { level: 1, name: "Assets" })).toBeVisible();
        await expect(page.getByTestId("chart-range-chips").first()).toBeVisible();
        await expectNoControlOverlap(page, undefined, at("Assets overview with range chips"));
        await page.goto("/");
        await expect(page.getByRole("heading", { level: 1, name: "Net worth" })).toBeVisible();
        await expectNoControlOverlap(page, undefined, at("dashboard with range chips"));
      }

      // Chip a11y and the remembered choice (#852, #901), at 390px from the loop
      // above. 15 months offers 1Y and All; the default 5Y is hidden, so All
      // reads as pressed.
      await page.goto(`/assets/bank-accounts/${ids.account}`);
      await expect(page.getByTestId("chart-range-5Y")).toHaveCount(0);
      await expect(page.getByTestId("chart-range-All")).toHaveAttribute("aria-pressed", "true");
      const chipBox = await page.getByTestId("chart-range-1Y").boundingBox();
      expect(chipBox).not.toBeNull();
      expect(chipBox!.height).toBeGreaterThanOrEqual(44);
      await page.getByTestId("chart-range-1Y").click();
      await expect(page.getByTestId("chart-range-1Y")).toHaveAttribute("aria-pressed", "true");
      await expect(page.getByTestId("chart-range-All")).toHaveAttribute("aria-pressed", "false");
      await page.reload();
      await expect(page.getByTestId("chart-range-1Y")).toHaveAttribute("aria-pressed", "true");

      // --- The Assets overview (#839 chart grid). The seed has one month, so
      // the share-over-time card doesn't render here; this guards the page's
      // controls only. ---
      for (const width of [1440, 768]) {
        await page.setViewportSize({ width, height: 900 });
        await page.goto("/assets");
        await expect(page.getByRole("heading", { level: 1, name: "Assets" })).toBeVisible();
        await expectNoControlOverlap(page, undefined, `Assets overview @${width}px`);
      }
    } finally {
      const cleanup = [
        ids.income && `/api/income/${ids.income}`,
        ids.account && `/api/bank-accounts/${ids.account}`,
        ids.stock && `/api/investments/stocks/${ids.stock}`,
        ids.deposit && `/api/investments/time-deposits/${ids.deposit}`,
        ids.bond && `/api/investments/bonds/${ids.bond}`,
        ids.tag && `/api/tags/${ids.tag}`,
      ].filter((u): u is string => !!u);
      for (const url of cleanup) {
        const res = await page.request.delete(url);
        expect.soft(res.ok(), `cleanup ${url}`).toBeTruthy();
      }
    }
  },
);

// covers: INV-PRESENTATION-26
// The signed-out sign-in card (#842): language corner control, provider button,
// tabs and form fields must not overlap at phone width. A fresh context with no
// session needs no fixtures.
test(
  "sign-in card has no overlapping controls at 390px",
  { tag: "@smoke" },
  async ({ browser }) => {
    const context = await browser.newContext({
      storageState: { cookies: [], origins: [] },
      viewport: { width: 390, height: 844 },
    });
    try {
      const page = await context.newPage();
      await page.goto("/");
      await expect(page.getByTestId("signin-language-select")).toBeVisible();
      await expect(page.getByTestId("local-auth-form")).toBeVisible();
      await expectNoControlOverlap(page, undefined, "sign-in card @390");
    } finally {
      await context.close();
    }
  },
);
