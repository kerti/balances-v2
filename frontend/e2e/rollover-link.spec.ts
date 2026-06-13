import { test, expect } from '@playwright/test'

// Manually linking a hand-created rollover successor (issue #65). When a TD
// matures with a rolled-over disposition, the source shows a "create rollover
// deposit" callout. If the user already created the successor by hand, the
// callout's "Already created it? Link existing" action lets them point the
// source at it — which stamps the successor's rolled_from on the backend,
// clears the callout, and surfaces the "rolled over into" link on the source's
// rollover card. Self-cleaning: deletes both TDs it creates. See ADR-0024.
test('link an existing time deposit as the rollover successor', async ({
  page,
}) => {
  const stamp = Date.now()
  const successorName = `E2E Rollover Succ ${stamp}`
  const sourceName = `E2E Rollover Src ${stamp}`

  await page.goto('/investments/time-deposits')

  // --- Create the hand-made successor first (unlinked, no_rollover default) ---
  await page.getByRole('button', { name: 'New time deposit' }).first().click()
  let dialog = page.getByRole('dialog')
  await expect(dialog.getByText('New time deposit')).toBeVisible()
  await dialog.getByLabel('Display name').fill(successorName)
  await dialog.getByLabel('Bank name').fill('E2E Bank')
  await dialog.getByLabel('Principal').fill('52750000')
  await dialog.getByLabel('Interest rate (% per year)').fill('4.5')
  await dialog.getByLabel('Term (months)').fill('12')
  await dialog.getByLabel('Placement date').fill('2026-01-01')
  await dialog.getByLabel('Maturity date').fill('2027-01-01')
  await dialog.getByLabel('Risk profile').selectOption('medium')
  await dialog.getByRole('button', { name: 'Create' }).click()
  await expect(
    page.getByRole('row', { name: new RegExp(successorName) }),
  ).toBeVisible()

  // --- Create the source TD that will roll over (auto_renew_with_interest so
  //     the Maturity dialog defaults both dispositions to rolled_to_new) ---
  await page.getByRole('button', { name: 'New time deposit' }).first().click()
  dialog = page.getByRole('dialog')
  await dialog.getByLabel('Display name').fill(sourceName)
  await dialog.getByLabel('Bank name').fill('E2E Bank')
  await dialog.getByLabel('Principal').fill('50000000')
  await dialog.getByLabel('Interest rate (% per year)').fill('4.5')
  await dialog.getByLabel('Term (months)').fill('12')
  await dialog.getByLabel('Placement date').fill('2025-01-01')
  await dialog.getByLabel('Maturity date').fill('2026-01-01')
  await dialog.getByLabel('At maturity').selectOption('auto_renew_with_interest')
  await dialog.getByLabel('Risk profile').selectOption('medium')
  await dialog.getByRole('button', { name: 'Create' }).click()

  // --- Open the source detail and record Maturity (rolled over) ---
  const sourceRow = page.getByRole('row', { name: new RegExp(sourceName) })
  await expect(sourceRow).toBeVisible()
  await sourceRow.getByText(sourceName).click()
  await expect(
    page.getByRole('heading', { level: 1, name: sourceName }),
  ).toBeVisible()

  await page.getByRole('button', { name: 'Maturity' }).click()
  const matDialog = page.getByRole('dialog')
  await expect(
    matDialog.getByRole('heading', { name: 'Record Maturity' }),
  ).toBeVisible()
  await matDialog.getByLabel('Principal (IDR)').fill('50000000')
  await matDialog.getByLabel('Interest (IDR)').fill('2750000')
  await matDialog.getByRole('button', { name: 'Record maturity' }).click()

  // The rollover callout appears (funds rolled, no successor linked yet).
  const callout = page.getByTestId('rollover-callout')
  await expect(callout).toBeVisible()

  // --- Link the existing hand-made successor via the callout action ---
  await page.getByTestId('rollover-link-trigger').click()
  const linkDialog = page.getByRole('dialog')
  await expect(
    linkDialog.getByText('Link the rollover deposit'),
  ).toBeVisible()
  await linkDialog
    .getByTestId('rollover-successor-select')
    .selectOption({ label: successorName })
  await linkDialog.getByTestId('rollover-link-submit').click()

  // Callout clears; the rollover card now links "into" the successor.
  await expect(page.getByTestId('rollover-callout')).toHaveCount(0)
  const intoLink = page.getByTestId('rollover-into-link')
  await expect(intoLink).toBeVisible()
  await expect(intoLink).toHaveText(successorName)

  // --- Cleanup: delete the source, then the successor ---
  await page.getByRole('button', { name: 'Delete' }).click()
  await page
    .getByRole('alertdialog')
    .getByRole('button', { name: 'Delete' })
    .click()
  await expect(page.getByText(sourceName)).toHaveCount(0)

  await page
    .getByRole('row', { name: new RegExp(successorName) })
    .getByText(successorName)
    .click()
  await expect(
    page.getByRole('heading', { level: 1, name: successorName }),
  ).toBeVisible()
  await page.getByRole('button', { name: 'Delete' }).click()
  await page
    .getByRole('alertdialog')
    .getByRole('button', { name: 'Delete' })
    .click()
  await expect(page.getByText(successorName)).toHaveCount(0)
})
