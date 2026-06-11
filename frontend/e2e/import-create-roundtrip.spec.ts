import { readFileSync } from 'node:fs'
import { test, expect } from '@playwright/test'

const XLSX_MIME =
  'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet'

// Create-from-list import round-trip (#88): export a bank account's full
// position workbook from its detail page, then feed that exact file through the
// list-screen "Import" dialog to create a brand-new, equivalent position —
// Detail sheet → the account, Snapshots sheet → its history, in one commit.
// Anchors on a unique display name; the import deliberately produces a second
// row with the same name (a copy), and we assert two exist before cleaning both
// up. See ADR-0024.
test('bank account export re-imports as a new position from the list', async ({
  page,
}) => {
  const name = `E2E import-create ${Date.now()}`
  const desc = `E2E import-create snap ${Date.now()}`

  await page.goto('/assets/bank-accounts')

  // --- Create the source bank account ---
  await page.getByRole('button', { name: 'New bank account' }).first().click()
  const acctDialog = page.getByRole('dialog')
  await acctDialog.getByLabel('Display name').fill(name)
  await acctDialog.getByLabel('Bank name').fill('E2E Bank')
  await acctDialog.getByLabel('Account number').fill('1234567890')
  await acctDialog.getByRole('button', { name: 'Create' }).click()

  // --- Open the detail page and record one snapshot ---
  await page
    .getByRole('row', { name: new RegExp(name) })
    .getByText(name)
    .click()
  await expect(page.getByRole('heading', { level: 1, name })).toBeVisible()

  await page.getByRole('button', { name: 'New' }).click()
  const snapDialog = page.getByRole('dialog')
  await expect(snapDialog.getByText('Record monthly snapshot')).toBeVisible()
  await snapDialog.getByLabel('Amount (IDR)').fill('12500000')
  await snapDialog.getByLabel('Description (optional)').fill(desc)
  await snapDialog.getByRole('button', { name: 'Save snapshot' }).click()
  await expect(page.getByRole('row', { name: new RegExp(desc) })).toBeVisible()

  // --- Export: a plain anchor download; capture the file ---
  const downloadPromise = page.waitForEvent('download')
  await page.getByTestId('bank-account-export').click()
  const download = await downloadPromise
  const filename = download.suggestedFilename()
  expect(filename).toMatch(/\.xlsx$/)
  // Re-attach the bytes under the real filename + MIME (Playwright's download
  // path is an extensionless temp file the .xlsx validator would reject).
  const buffer = readFileSync(await download.path())

  // --- Back to the list and import the workbook as a NEW position ---
  await page.goto('/assets/bank-accounts')
  await page.getByTestId('import-position-trigger').click()
  await page
    .getByTestId('import-file-input')
    .setInputFiles({ name: filename, mimeType: XLSX_MIME, buffer })
  await expect(page.getByTestId('import-selected-file')).toBeVisible()

  // Dry-run check: the workbook is clean (no field/row errors) and would create.
  await page.getByTestId('import-check-btn').click()
  await expect(page.getByTestId('import-result')).toBeVisible()
  await expect(page.getByTestId('import-errors')).toHaveCount(0)

  // Clean preview lights up the create button; committing creates the position.
  const commit = page.getByTestId('import-commit-btn')
  await expect(commit).toBeEnabled()
  await commit.click()
  await expect(page.getByTestId('import-done')).toBeVisible()
  await page.getByRole('button', { name: 'Done' }).click()

  // Two same-named rows now exist: the source and the imported copy.
  await expect(page.getByRole('row', { name: new RegExp(name) })).toHaveCount(2)

  // --- Cleanup: delete both accounts (each delete returns to the list) ---
  for (let i = 0; i < 2; i++) {
    await page
      .getByRole('row', { name: new RegExp(name) })
      .first()
      .getByText(name)
      .click()
    await page.getByRole('button', { name: 'Delete' }).click()
    const confirm = page.getByRole('alertdialog')
    await confirm.getByRole('button', { name: 'Delete' }).click()
    await expect(page.getByRole('row', { name: new RegExp(name) })).toHaveCount(
      1 - i,
    )
  }
})
