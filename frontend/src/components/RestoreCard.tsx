import { useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { errorMessage } from '@/lib/errorMessage'
import {
  isHouseholdEmpty,
  postRestoreCommit,
  postRestorePreview,
  totalRows,
  type RestorePreview,
} from '@/lib/backup'

// RestoreCard is the import half of Settings → Data (ADR-0036, issue #175). It
// is a two-step, deliberately heavy flow: pick a backup → preview the stakes
// (what loads, what gets erased) → confirm at a ceremony scaled to those stakes
// (a checkbox when the current household is empty, type-to-erase when it holds
// data) → commit. The commit wipes the caller's session, so on success the app
// reloads into the sign-in screen; the next Google login re-links by google_sub.
export function RestoreCard() {
  const { t } = useTranslation('settings')
  const fileRef = useRef<HTMLInputElement>(null)
  const [file, setFile] = useState<File | null>(null)
  const [preview, setPreview] = useState<RestorePreview | null>(null)
  const [previewing, setPreviewing] = useState(false)
  const [committing, setCommitting] = useState(false)
  const [ack, setAck] = useState(false)
  const [eraseInput, setEraseInput] = useState('')

  const eraseWord = t('data.restore.eraseWord')

  const reset = () => {
    setFile(null)
    setPreview(null)
    setAck(false)
    setEraseInput('')
    if (fileRef.current) fileRef.current.value = ''
  }

  const handlePick = async (picked: File) => {
    setFile(picked)
    setPreview(null)
    setAck(false)
    setEraseInput('')
    setPreviewing(true)
    try {
      setPreview(await postRestorePreview(picked))
    } catch (err) {
      toast.error(errorMessage(err, t('data.restore.previewError')))
      reset()
    } finally {
      setPreviewing(false)
    }
  }

  const stakesEmpty = preview ? isHouseholdEmpty(preview.current) : true
  const confirmed = stakesEmpty
    ? ack
    : eraseInput.trim().toLocaleUpperCase() === eraseWord.toLocaleUpperCase()

  const handleCommit = async () => {
    if (!file || !confirmed) return
    setCommitting(true)
    try {
      await postRestoreCommit(file)
      toast.success(t('data.restore.done'))
      // The server re-issued our session, so we stay signed in. A full load of the
      // dashboard is the cleanest way to surface the wholly-replaced data set.
      setTimeout(() => window.location.assign('/'), 1200)
    } catch (err) {
      setCommitting(false)
      toast.error(errorMessage(err, t('data.restore.failed')))
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{t('data.restore.title')}</CardTitle>
        <CardDescription>{t('data.restore.description')}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <input
          ref={fileRef}
          type="file"
          accept=".json,.gz,application/gzip,application/json"
          className="hidden"
          data-testid="restore-file-input"
          onChange={(e) => {
            const f = e.target.files?.[0]
            if (f) void handlePick(f)
          }}
        />

        {!preview && (
          <div className="flex items-center gap-3">
            <Button
              variant="outline"
              onClick={() => fileRef.current?.click()}
              disabled={previewing}
              data-testid="restore-choose-button"
            >
              {previewing ? t('data.restore.checking') : t('data.restore.choose')}
            </Button>
            <p className="text-sm text-muted-foreground">{t('data.restore.chooseHint')}</p>
          </div>
        )}

        {preview && (
          <div className="space-y-4" data-testid="restore-preview">
            <div className="rounded-md border p-3 text-sm">
              <p>
                {t('data.restore.summary', {
                  household: preview.backup.household_name,
                  count: totalRows(preview.backup.counts),
                })}
              </p>
            </div>

            <div
              className="rounded-md border border-destructive/50 bg-destructive/5 p-3 text-sm"
              data-testid="restore-stakes"
            >
              {stakesEmpty ? (
                <p>{t('data.restore.stakesEmpty')}</p>
              ) : (
                <p className="font-medium text-destructive">
                  {t('data.restore.stakesWarning', {
                    count: totalRows(preview.current),
                  })}
                </p>
              )}
            </div>

            {stakesEmpty ? (
              <label className="flex items-start gap-2 text-sm">
                <input
                  type="checkbox"
                  className="mt-1 h-4 w-4"
                  checked={ack}
                  disabled={committing}
                  onChange={(e) => setAck(e.target.checked)}
                  data-testid="restore-ack-checkbox"
                />
                <span>{t('data.restore.ack')}</span>
              </label>
            ) : (
              <div className="space-y-1">
                <label htmlFor="restore-erase" className="text-sm">
                  {t('data.restore.erasePrompt', { word: eraseWord })}
                </label>
                <Input
                  id="restore-erase"
                  value={eraseInput}
                  disabled={committing}
                  autoComplete="off"
                  onChange={(e) => setEraseInput(e.target.value)}
                  data-testid="restore-erase-input"
                />
              </div>
            )}

            <div className="flex items-center gap-3">
              <Button
                variant="destructive"
                onClick={handleCommit}
                disabled={!confirmed || committing}
                data-testid="restore-commit-button"
              >
                {committing ? t('data.restore.restoring') : t('data.restore.commit')}
              </Button>
              <Button variant="ghost" onClick={reset} disabled={committing}>
                {t('data.restore.cancel')}
              </Button>
            </div>

            {committing && (
              <p className="text-sm text-muted-foreground" data-testid="restore-progress">
                {t('data.restore.dontClose')}
              </p>
            )}
          </div>
        )}
      </CardContent>
    </Card>
  )
}
