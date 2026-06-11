import { readFileSync } from 'node:fs'
import { test, expect } from '@playwright/test'

const XLSX_MIME =
  'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet'

// Export → re-import round-trip for the per-position workbook (#85/#86). On a
// property detail page (representative of the property/vehicle/liability/
// receivable export fan-out) we record a snapshot, download the export, then
// feed that exact file back through the snapshot-import dialog and confirm it
// re-imports flawlessly — clean preview (the month re-classifies as an update,
// not an insert) and a committed result with no row errors. Anchors on a unique
// display name + description; self-cleaning. See ADR-0024.
test('property export re-imports flawlessly', async ({ page }) => {
  const name = `E2E export prop ${Date.now()}`
  const desc = `E2E export snap ${Date.now()}`

  await page.goto('/assets/properties')

  // --- Create the parent property (display name only; type/currency default) ---
  await page.getByRole('button', { name: 'New property' }).first().click()
  const createDialog = page.getByRole('dialog')
  await expect(createDialog.getByText('New property')).toBeVisible()
  await createDialog.getByLabel('Display name').fill(name)
  await createDialog.getByRole('button', { name: 'Create' }).click()

  // --- Open the detail page ---
  await page.getByRole('row', { name: new RegExp(name) }).getByText(name).click()
  await expect(page.getByRole('heading', { level: 1, name })).toBeVisible()

  // --- Record one snapshot (month defaults to the current month) ---
  await page.getByRole('button', { name: 'New' }).click()
  const snapDialog = page.getByRole('dialog')
  await expect(snapDialog.getByText('Record monthly snapshot')).toBeVisible()
  await snapDialog.getByLabel('Amount (IDR)').fill('2500000000')
  await snapDialog.getByLabel('Description (optional)').fill(desc)
  await snapDialog.getByRole('button', { name: 'Save snapshot' }).click()
  await expect(page.getByRole('row', { name: new RegExp(desc) })).toBeVisible()

  // --- Export: the button is a plain anchor download; capture the file ---
  const downloadPromise = page.waitForEvent('download')
  await page.getByTestId('property-export').click()
  const download = await downloadPromise
  const filename = download.suggestedFilename()
  expect(filename).toMatch(/\.xlsx$/)

  // Read the bytes and re-attach them under the real filename + MIME. Playwright's
  // download.path() points at a GUID-named temp file with no extension, which the
  // dialog's .xlsx validator (lib/importDrop) would reject — so feed a buffer with
  // the proper name instead, exactly as a user re-selecting the file would.
  const buffer = readFileSync(await download.path())

  // --- Feed the exported file straight back through the import dialog ---
  await page.getByTestId('import-snapshots-trigger').click()
  await page
    .getByTestId('import-file-input')
    .setInputFiles({ name: filename, mimeType: XLSX_MIME, buffer })
  await expect(page.getByTestId('import-selected-file')).toBeVisible()

  // Dry-run check: the file is clean, and the one month re-classifies as an
  // update (not an insert) since the snapshot already exists — i.e. a true
  // round trip, not a fresh write.
  await page.getByTestId('import-check-btn').click()
  const result = page.getByTestId('import-result')
  await expect(result).toBeVisible()
  await expect(result).toContainText('update')
  await expect(page.getByTestId('import-errors')).toHaveCount(0)

  // Clean preview lights up the commit button; importing succeeds with no errors.
  const commit = page.getByTestId('import-commit-btn')
  await expect(commit).toBeEnabled()
  await commit.click()
  await expect(page.getByTestId('import-done')).toBeVisible()

  await page.getByRole('button', { name: 'Done' }).click()

  // The snapshot survived the round trip unchanged.
  await expect(page.getByRole('row', { name: new RegExp(desc) })).toBeVisible()

  // --- Delete the parent property (cleanup — returns to the empty list) ---
  await page.getByRole('button', { name: 'Delete' }).click()
  const confirm = page.getByRole('alertdialog')
  await confirm.getByRole('button', { name: 'Delete' }).click()
  await expect(page.getByText(name)).toHaveCount(0)
})
