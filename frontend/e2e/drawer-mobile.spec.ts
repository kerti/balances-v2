import { test, expect } from "@playwright/test";

// The off-canvas drawer below the 1024px shell boundary (ADR-0050 amendment,
// #837, #849): 390px is the phone, 900px the tablet band that has the drawer
// with desktop content. Both get 44px links (#559), a close ×, and an account
// section carrying Sign out, which leaves the header there. From 1024px the
// header keeps Sign out and no drawer account section exists.
// covers: INV-PRESENTATION-08
for (const width of [390, 900]) {
  test(
    `the drawer at ${width}px has 44px links, a close x and the account section`,
    { tag: "@smoke" },
    async ({ page }) => {
      await page.setViewportSize({ width, height: 844 });
      await page.goto("/settings");
      await expect(page.getByRole("heading", { level: 1, name: "Settings" })).toBeVisible();

      // Sign out is not in the header at this width.
      await expect(page.getByTestId("header-sign-out")).toBeHidden();

      await page.getByRole("button", { name: "Toggle Sidebar" }).click();
      const drawer = page.getByRole("dialog");
      await expect(drawer).toBeVisible();

      // Every nav link clears the tap floor, main items and sub-items alike.
      for (const name of ["Dashboard", "Settings", "Bank accounts", "Time deposits"]) {
        const link = drawer.getByRole("link", { name, exact: true });
        await link.scrollIntoViewIfNeeded();
        const box = await link.boundingBox();
        expect(box, `${name} link should render`).not.toBeNull();
        expect(box!.height, `${name} link height`).toBeGreaterThanOrEqual(44);
      }

      // Account section: name, email and Sign out, at the bottom of the drawer.
      await expect(drawer.getByTestId("drawer-account-name")).not.toBeEmpty();
      await expect(drawer.getByTestId("drawer-account-email")).not.toBeEmpty();
      const signOut = drawer.getByTestId("drawer-sign-out");
      await expect(signOut).toBeVisible();
      await expect(signOut).toBeInViewport();
      const signOutBox = await signOut.boundingBox();
      expect(signOutBox!.height).toBeGreaterThanOrEqual(44);

      // The footer sits below the nav and never covers the last item: the
      // drawer's links stay clear of the account section.
      const settings = drawer.getByRole("link", { name: "Settings", exact: true });
      await settings.scrollIntoViewIfNeeded();
      const settingsBox = await settings.boundingBox();
      const accountBox = await drawer.getByTestId("drawer-account").boundingBox();
      expect(settingsBox!.y + settingsBox!.height).toBeLessThanOrEqual(accountBox!.y + 1);

      // The close × is a 44px target and closes the drawer.
      const close = drawer.getByTestId("drawer-close");
      const closeBox = await close.boundingBox();
      expect(closeBox!.width).toBeGreaterThanOrEqual(44);
      expect(closeBox!.height).toBeGreaterThanOrEqual(44);
      await close.click();
      await expect(drawer).toHaveCount(0);
    },
  );
}

// covers: INV-PRESENTATION-08
test(
  "from 1024px Sign out is in the header and there is no drawer",
  { tag: "@smoke" },
  async ({ page }) => {
    await page.setViewportSize({ width: 1024, height: 800 });
    await page.goto("/settings");
    await expect(page.getByRole("heading", { level: 1, name: "Settings" })).toBeVisible();
    await expect(page.getByTestId("header-sign-out")).toBeVisible();
    await expect(page.getByTestId("drawer-account")).toHaveCount(0);
    await expect(page.getByRole("button", { name: "Toggle Sidebar" })).toBeHidden();
  },
);
