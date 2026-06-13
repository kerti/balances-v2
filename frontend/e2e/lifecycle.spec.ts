import { test, expect } from '@playwright/test'

// Position lifecycle through the real UI + backend (M4.6, ADR-0009): create a
// bank account, close it (active → closed via the dedicated Terminate dialog),
// then reopen it (closed → active correction). Asserts the two observable
// consequences of the biconditional status=active ⟺ terminated_at IS NULL:
// the StatusBadge flips Active⇄Closed, and the "+ New snapshot" button is gated
// off on a non-active position and returns on reopen. Self-cleaning: deletes
// the account it creates, leaving the seed's empty bank-account list. See
// ADR-0024.
test('bank account lifecycle: close → reopen → delete', { tag: '@smoke' }, async ({ page }) => {
  const name = `E2E account ${Date.now()}`
  // The position's lifecycle pill, by test id. The Terminate dialog's <select>
  // carries <option>s with the same labels, so a text or tag locator would be
  // ambiguous; the test id pins us to the StatusBadge.
  const statusBadge = page.getByTestId('status-badge')

  await page.goto('/assets/bank-accounts')

  // --- Create (minimal required fields; currency/type/ownership default) ---
  await page.getByRole('button', { name: 'New bank account' }).first().click()
  const createDialog = page.getByRole('dialog')
  await expect(createDialog.getByText('New bank account')).toBeVisible()
  await createDialog.getByLabel('Display name').fill(name)
  await createDialog.getByLabel('Bank name').fill('E2E Bank')
  await createDialog.getByLabel('Account number').fill('1234567890')
  await createDialog.getByRole('button', { name: 'Create' }).click()

  // --- Navigate to the detail page ---
  const row = page.getByRole('row', { name: new RegExp(name) })
  await expect(row).toBeVisible()
  await row.getByText(name).click()

  await expect(
    page.getByRole('heading', { level: 1, name }),
  ).toBeVisible()
  // Active position: badge muted-active, snapshot entry available.
  await expect(statusBadge).toHaveText('Active')
  await expect(
    page.getByRole('button', { name: 'New' }),
  ).toBeVisible()

  // --- Close (active → closed; date auto-fills today, note optional) ---
  await page.getByRole('button', { name: 'Close', exact: true }).click()
  const closeDialog = page.getByRole('dialog')
  await expect(closeDialog.getByText('Close position')).toBeVisible()
  await closeDialog.getByLabel('Status').selectOption('closed')
  await closeDialog.getByLabel('Note (optional)').fill('E2E closed')
  await closeDialog.getByRole('button', { name: 'Save' }).click()

  // Badge flips to Closed; snapshot entry is gated off on a terminated position.
  await expect(statusBadge).toHaveText('Closed')
  await expect(
    page.getByRole('button', { name: 'New' }),
  ).toHaveCount(0)

  // --- Reopen (closed → active correction; biconditional clears the date) ---
  await page.getByRole('button', { name: 'Status', exact: true }).click()
  const reopenDialog = page.getByRole('dialog')
  await expect(reopenDialog.getByText('Edit lifecycle status')).toBeVisible()
  await reopenDialog.getByLabel('Status').selectOption('active')
  await reopenDialog.getByRole('button', { name: 'Save' }).click()

  await expect(statusBadge).toHaveText('Active')
  await expect(
    page.getByRole('button', { name: 'New' }),
  ).toBeVisible()

  // --- Delete (cleanup — returns to the list, leaving it empty) ---
  await page.getByRole('button', { name: 'Delete' }).click()
  const confirm = page.getByRole('alertdialog')
  await confirm.getByRole('button', { name: 'Delete' }).click()

  await expect(page.getByText(name)).toHaveCount(0)
})
