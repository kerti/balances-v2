import { test, expect } from '@playwright/test'

// Settings → Appearance round-trip. Order-independent by design: rather than
// asserting a seed "default" (users.theme is mutable, so a partial re-run could
// leave it either way), it drives the theme to each value and proves the choice
// applies live (the `dark` class on <html>) and survives a reload (the boot
// script in index.html re-applies it from the server session before paint).
// Ends on dark so the suite's shared Alice row is left in the global-setup-
// pinned state. See #33.
//
// The selection is buttonless autosave (#54/ADR-0032): selectOption fires a
// PATCH /api/me whose value the post-reload boot reads back. So each reload must
// wait for that PATCH to land first — selecting and reloading immediately races
// the in-flight write and reads the stale server value.
test('settings theme round-trip: light and dark both apply and persist', async ({
  page,
}) => {
  await page.goto('/settings')

  const select = page.getByTestId('settings-theme-select')
  await expect(select).toBeVisible()

  // selectTheme drives the select and resolves only once the persisting
  // PATCH /api/me has succeeded, so a following reload reads the saved value.
  const selectTheme = async (value: 'light' | 'dark') => {
    await Promise.all([
      page.waitForResponse(
        (r) =>
          r.url().includes('/api/me') &&
          r.request().method() === 'PATCH' &&
          r.ok(),
      ),
      page.getByTestId('settings-theme-select').selectOption(value),
    ])
  }

  // --- Light: applies live, persists past reload ---
  await selectTheme('light')
  await expect(page.locator('html')).not.toHaveClass(/dark/)

  await page.reload()
  await expect(page.getByTestId('settings-theme-select')).toHaveValue('light')
  await expect(page.locator('html')).not.toHaveClass(/dark/)

  // --- Dark: applies live, persists past reload (self-cleaning) ---
  await selectTheme('dark')
  await expect(page.locator('html')).toHaveClass(/dark/)

  await page.reload()
  await expect(page.getByTestId('settings-theme-select')).toHaveValue('dark')
  await expect(page.locator('html')).toHaveClass(/dark/)
})
