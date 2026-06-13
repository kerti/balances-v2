import { test, expect } from '@playwright/test'

// Proves the session-injection harness end-to-end: the storageState cookie is
// accepted by the real SessionMiddleware, so the app renders authenticated as
// Alice (the seeded fixture user) instead of the sign-in screen. See ADR-0024.
test('injected session renders the authenticated shell as Alice', { tag: '@smoke' }, async ({
  page,
}) => {
  await page.goto('/')

  // AppShell header (authenticated): display name + email + Sign out.
  await expect(page.getByText('Alice', { exact: true })).toBeVisible()
  await expect(page.getByText('alice@example.com')).toBeVisible()
  await expect(page.getByRole('button', { name: 'Sign out' })).toBeVisible()

  // SignInScreen must not be present.
  await expect(
    page.getByRole('link', { name: /sign in with google/i }),
  ).toHaveCount(0)
})
