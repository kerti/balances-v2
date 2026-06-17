import { test, expect } from '@playwright/test'

// Settings → Data → Restore (ADR-0036, issue #175). Seeded Alice is
// auto-authenticated (global-setup) and her household is non-empty, so the
// confirmation escalates to type-to-erase. We export a real backup, feed it back
// into the restore picker, and assert the preview + stakes + confirm-gating
// wiring end-to-end (the upload hits the real /restore/preview endpoint). We stop
// short of committing on purpose: a commit destructively replaces the shared
// seeded household and would knock over every other @smoke test — the wipe+load
// (and the post-restore session re-issue) is covered by the Go integration tests.
//
// covers: INV-BACKUP-08
test('settings restore previews a backup and gates the destructive commit', { tag: '@smoke' }, async ({ page }) => {
  await page.goto('/settings')

  // Grab a real backup file via the export button.
  const downloadPromise = page.waitForEvent('download')
  await page.getByTestId('backup-export-button').click()
  const download = await downloadPromise
  const path = await download.path()

  // Feed it back into the restore picker (the input is visually hidden).
  await page.getByTestId('restore-file-input').setInputFiles(path)

  // Preview appears, and because Alice's household holds data the stakes warning
  // and the type-to-erase ceremony are shown (not the empty-household checkbox).
  await expect(page.getByTestId('restore-preview')).toBeVisible()
  await expect(page.getByTestId('restore-stakes')).toBeVisible()
  const eraseInput = page.getByTestId('restore-erase-input')
  await expect(eraseInput).toBeVisible()

  // The destructive commit stays disabled until the erase word is typed exactly.
  const commitBtn = page.getByTestId('restore-commit-button')
  await expect(commitBtn).toBeDisabled()
  await eraseInput.fill('nope')
  await expect(commitBtn).toBeDisabled()
  await eraseInput.fill('ERASE')
  await expect(commitBtn).toBeEnabled()
})
