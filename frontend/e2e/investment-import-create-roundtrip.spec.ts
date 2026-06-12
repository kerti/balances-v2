import { readFileSync } from 'node:fs'
import { test, expect } from '@playwright/test'

const XLSX_MIME =
  'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet'

// Create-from-list import round-trip for an investment WITH a ledger (#90, parent
// #51). On a stock detail page (representative of the five subtypes) we record a
// Buy transaction and a quantity-price snapshot, download the full position
// workbook (Detail + Snapshots + Transactions, ADR-0023), then feed that exact
// file through the list-screen "Import" dialog. Unlike the detail-page snapshot
// import — which reads only the Snapshots sheet — the list-screen import seeds all
// three: a brand-new stock from Detail, its history from Snapshots, AND its ledger
// from Transactions, in one commit. We assert the imported copy carries the seeded
// Buy, the #90 differentiator over the snapshot-only bank round-trip (#88).
// Anchors on a unique display name; self-cleaning. See ADR-0024.
test('stock export re-imports as a new position with its ledger from the list', async ({
  page,
}) => {
  const name = `E2E import-create inv ${Date.now()}`
  const desc = `E2E import-create snap ${Date.now()}`

  await page.goto('/investments/stocks')

  // --- Create the source stock position ---
  await page.getByRole('button', { name: 'New stock' }).first().click()
  const createDialog = page.getByRole('dialog')
  await expect(createDialog.getByText('New stock position')).toBeVisible()
  await createDialog.getByLabel('Display name').fill(name)
  await createDialog.getByLabel('Ticker').fill('E2EX')
  await createDialog.getByLabel('Exchange').fill('IDX')
  await createDialog.getByLabel('Risk profile').selectOption('medium')
  await createDialog.getByRole('button', { name: 'Create' }).click()

  // --- Open the detail page ---
  await page.getByRole('row', { name: new RegExp(name) }).getByText(name).click()
  await expect(page.getByRole('heading', { level: 1, name })).toBeVisible()

  // --- Record a Buy so the export's Transactions sheet has a real ledger row ---
  await page.getByRole('button', { name: 'Buy' }).click()
  const buyDialog = page.getByRole('dialog')
  await expect(
    buyDialog.getByRole('heading', { name: 'Record Buy' }),
  ).toBeVisible()
  await buyDialog.getByLabel('Quantity (sh)').fill('100')
  await buyDialog.getByLabel('Price per unit (IDR)').fill('8500')
  await buyDialog.getByRole('button', { name: 'Record buy' }).click()
  await expect(page.getByRole('row', { name: /Buy/ })).toBeVisible()

  // --- Record one quantity-price snapshot (month defaults to current) ---
  await page.getByRole('button', { name: 'New' }).click()
  const snapDialog = page.getByRole('dialog')
  await expect(snapDialog.getByText('Record monthly snapshot')).toBeVisible()
  await snapDialog.getByLabel('Quantity', { exact: true }).fill('100')
  await snapDialog.getByLabel('Price per unit (IDR)').fill('8500')
  await snapDialog.getByLabel('Description (optional)').fill(desc)
  await snapDialog.getByRole('button', { name: 'Save snapshot' }).click()
  await expect(page.getByRole('row', { name: new RegExp(desc) })).toBeVisible()

  // --- Export: the button is a plain anchor download; capture the file ---
  const downloadPromise = page.waitForEvent('download')
  await page.getByTestId('stock-export').click()
  const download = await downloadPromise
  const filename = download.suggestedFilename()
  expect(filename).toMatch(/\.xlsx$/)
  // Re-attach the bytes under the real filename + MIME (Playwright's download
  // path is an extensionless temp file the .xlsx validator would reject).
  const buffer = readFileSync(await download.path())

  // --- Back to the list and import the workbook as a NEW position ---
  await page.goto('/investments/stocks')
  await page.getByTestId('import-position-trigger').click()
  await page
    .getByTestId('import-file-input')
    .setInputFiles({ name: filename, mimeType: XLSX_MIME, buffer })
  await expect(page.getByTestId('import-selected-file')).toBeVisible()

  // Dry-run check: the workbook is clean (no field/row errors) and would create.
  await page.getByTestId('import-check-btn').click()
  await expect(page.getByTestId('import-result')).toBeVisible()
  await expect(page.getByTestId('import-errors')).toHaveCount(0)

  // Clean preview lights up the create button; committing creates the position
  // plus its seeded snapshots and ledger.
  const commit = page.getByTestId('import-commit-btn')
  await expect(commit).toBeEnabled()
  await commit.click()
  await expect(page.getByTestId('import-done')).toBeVisible()
  await page.getByRole('button', { name: 'Done' }).click()

  // Two same-named rows now exist: the source and the imported copy.
  await expect(page.getByRole('row', { name: new RegExp(name) })).toHaveCount(2)

  // The imported copy carries the seeded ledger: open one and confirm the Buy
  // row is present (the #90 differentiator — the list import seeds transactions,
  // not just snapshots).
  await page
    .getByRole('row', { name: new RegExp(name) })
    .first()
    .getByText(name)
    .click()
  await expect(page.getByRole('heading', { level: 1, name })).toBeVisible()
  await expect(page.getByRole('row', { name: /Buy/ })).toBeVisible()

  // --- Cleanup: delete both stocks (each delete returns to the list) ---
  await page.goto('/investments/stocks')
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
