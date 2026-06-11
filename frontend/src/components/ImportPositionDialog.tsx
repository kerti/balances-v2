import { useRef, useState } from 'react'
import { Upload } from 'lucide-react'
import type { UseMutationResult } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'
import { errorMessage } from '@/lib/errorMessage'
import { fileFromDrop } from '@/lib/importDrop'
import { cn } from '@/lib/utils'
import type { CreateImportArgs, CreateImportResult } from '@/hooks/snapshotImport'

type Props = {
  // noun + mutation are owned by the parent so the same dialog can drive
  // create-from-file import for any position group (bank account first).
  noun: string
  mutation: UseMutationResult<CreateImportResult, unknown, CreateImportArgs>
}

// Create a brand-new position from a workbook: its Detail sheet becomes the
// position, its Snapshots sheet seeds the history. "Check file" runs a
// server-side dry-run (validates Detail fields + snapshot rows, writes nothing);
// "Create" only lights up once the file is clean. The format is a position
// export — the dialog tells the user to export an existing one to get it.
export function ImportPositionDialog({ noun, mutation }: Props) {
  const { t } = useTranslation('common')
  const [open, setOpen] = useState(false)
  const [file, setFile] = useState<File | null>(null)
  const [result, setResult] = useState<CreateImportResult | null>(null)
  const [dragActive, setDragActive] = useState(false)
  const [invalid, setInvalid] = useState(false)
  const fileRef = useRef<HTMLInputElement>(null)

  function reset() {
    setFile(null)
    setResult(null)
    setDragActive(false)
    setInvalid(false)
    mutation.reset()
    if (fileRef.current) fileRef.current.value = ''
  }
  function close() {
    setOpen(false)
    reset()
  }

  // Single funnel for both the picker and the drop zone so a non-.xlsx is
  // rejected identically either way. An `empty` drop leaves the selection as-is.
  function selectFiles(files: FileList | File[] | null) {
    const outcome = fileFromDrop(files)
    if (!outcome.ok && outcome.reason === 'empty') return
    setResult(null)
    mutation.reset()
    if (!outcome.ok) {
      setFile(null)
      setInvalid(true)
      if (fileRef.current) fileRef.current.value = ''
      return
    }
    setInvalid(false)
    setFile(outcome.file)
  }

  function onPickFile(e: React.ChangeEvent<HTMLInputElement>) {
    selectFiles(e.target.files)
  }

  function onDragOver(e: React.DragEvent) {
    e.preventDefault()
    setDragActive(true)
  }
  function onDragLeave(e: React.DragEvent) {
    e.preventDefault()
    setDragActive(false)
  }
  function onDrop(e: React.DragEvent) {
    e.preventDefault()
    setDragActive(false)
    selectFiles(e.dataTransfer.files)
  }

  function run(mode: 'preview' | 'commit') {
    if (!file) return
    mutation.mutate({ file, mode }, { onSuccess: setResult })
  }

  const fieldErrors = result?.field_errors ?? []
  const rowErrors = result?.errors ?? []
  const hasErrors = fieldErrors.length > 0 || rowErrors.length > 0
  const cleanPreview = result?.mode === 'preview' && result.would_create
  const committed = result?.committed === true

  return (
    <Dialog open={open} onOpenChange={(o) => (o ? setOpen(true) : close())}>
      <DialogTrigger asChild>
        <Button size="sm" variant="outline" data-testid="import-position-trigger">
          <Upload className="mr-1 size-4" />
          {t('importCreate.trigger')}
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('importCreate.title', { noun })}</DialogTitle>
          <DialogDescription>
            {t('importCreate.description', { noun })}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          <div className="grid gap-1.5">
            <Label htmlFor="import-create-file">{t('importCreate.uploadLabel')}</Label>
            <div
              onDragOver={onDragOver}
              onDragLeave={onDragLeave}
              onDrop={onDrop}
              data-testid="import-drop-zone"
              data-drag-active={dragActive}
              className={cn(
                'grid gap-2 rounded-md border-2 border-dashed p-4 transition-colors',
                dragActive
                  ? 'border-primary bg-primary/5'
                  : 'border-input bg-muted/30',
              )}
            >
              <p className="text-sm text-muted-foreground">
                {dragActive ? t('import.dropActive') : t('import.dropHint')}
              </p>
              <input
                id="import-create-file"
                ref={fileRef}
                type="file"
                accept=".xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
                onChange={onPickFile}
                className="text-sm"
                data-testid="import-file-input"
              />
            </div>
            <p className="text-xs text-muted-foreground">
              {t('importCreate.formatHint', { noun })}
            </p>
            {invalid && (
              <p className="text-sm text-destructive" data-testid="import-invalid-file">
                {t('import.invalidFile')}
              </p>
            )}
            {file && !invalid && (
              <p className="text-xs text-muted-foreground" data-testid="import-selected-file">
                {file.name}
              </p>
            )}
          </div>

          {result && !committed && (
            <div className="text-sm" data-testid="import-result">
              {hasErrors ? (
                <div className="space-y-1">
                  <p className="text-destructive">
                    {t('importCreate.needsFixing', {
                      count: fieldErrors.length + rowErrors.length,
                    })}
                  </p>
                  <ul
                    className="list-disc space-y-0.5 pl-5 text-destructive"
                    data-testid="import-errors"
                  >
                    {fieldErrors.map((e) => (
                      <li key={`f-${e.field}`}>
                        {t('importCreate.fieldError', {
                          field: e.field,
                          message: e.message,
                        })}
                      </li>
                    ))}
                    {rowErrors.map((e) => (
                      <li key={`r-${e.row}`}>
                        {t('import.rowError', { row: e.row, message: e.message })}
                      </li>
                    ))}
                  </ul>
                </div>
              ) : (
                <p className="text-muted-foreground">
                  {t('importCreate.preview', { noun, count: result.to_insert })}
                </p>
              )}
            </div>
          )}

          {committed && (
            <p className="text-sm text-emerald-600" data-testid="import-done">
              {t('importCreate.committed', { noun, count: result.to_insert })}
            </p>
          )}

          {mutation.isError && (
            <p className="text-sm text-destructive">
              {errorMessage(mutation.error)}
            </p>
          )}
        </div>

        <DialogFooter>
          {committed ? (
            <Button type="button" onClick={close}>
              {t('actions.done')}
            </Button>
          ) : (
            <>
              <Button type="button" variant="outline" onClick={close}>
                {t('cancel')}
              </Button>
              <Button
                type="button"
                variant="outline"
                disabled={!file || mutation.isPending}
                onClick={() => run('preview')}
                data-testid="import-check-btn"
              >
                {mutation.isPending && !committed
                  ? t('actions.checking')
                  : t('import.checkFile')}
              </Button>
              <Button
                type="button"
                disabled={!cleanPreview || mutation.isPending}
                onClick={() => run('commit')}
                data-testid="import-commit-btn"
              >
                {t('importCreate.createBtn')}
              </Button>
            </>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
