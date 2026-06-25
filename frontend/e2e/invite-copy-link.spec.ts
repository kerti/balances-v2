import { test, expect } from '@playwright/test'

// Grant clipboard read+write so navigator.clipboard resolves in headless
// Chromium (localhost is a secure context; the permission is the missing piece).
test.use({ permissions: ['clipboard-read', 'clipboard-write'] })

// covers: INV-NOTIFICATIONS-10
// covers: INV-NOTIFICATIONS-11
// The "copy invite link" affordance — the fallback that makes invitations work
// with EMAIL_ENABLED=false (ADR-0037), where the invite email never sends. The
// create endpoint returns the AcceptURL regardless, so the inviter copies the
// link and shares it by hand. Creates a real invitation through the UI as the
// seeded Alice, then exercises copy: the button flips to "Copied!" and the
// clipboard holds the same accept URL shown on screen.
//
// Also pins the no-mail half of INV-NOTIFICATIONS-11: the e2e backend runs the
// NoopMailer (EMAIL_ENABLED=false), so Send succeeds and email_sent=true — the
// inviter must NOT be nagged with the "couldn't be sent" toast here; the
// copy-link panel is the affordance. The email_sent=false toast itself can't be
// exercised against this real stack (NoopMailer never errors); the backend
// best-effort test guards that branch.
test('invite flow surfaces a copyable accept link', { tag: '@smoke' }, async ({
  page,
}) => {
  await page.goto('/settings')

  const inviteEmail = `e2e-invitee-${Date.now()}@example.com`
  await page.getByLabel('Email address').fill(inviteEmail)
  await page.getByRole('button', { name: 'Send invitation' }).click()

  // Result block: the accept URL is rendered for manual sharing.
  await expect(page.getByText(`Invitation sent to ${inviteEmail}`)).toBeVisible()
  const acceptUrl = await page.getByTestId('invite-accept-url').innerText()
  expect(acceptUrl).toContain('invite=')

  const copyButton = page.getByTestId('copy-invite-link')
  await expect(copyButton).toBeVisible()
  await copyButton.click()
  await expect(copyButton).toHaveText('Copied!')

  const clipboard = await page.evaluate(() => navigator.clipboard.readText())
  expect(clipboard).toBe(acceptUrl)

  // email_sent=true (NoopMailer): no failed-send toast nags the inviter.
  await expect(
    page.getByText("the email couldn't be sent", { exact: false }),
  ).toHaveCount(0)
})
