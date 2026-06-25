import { useRef, useState } from "react";
import { Upload } from "lucide-react";
import type { UseMutationResult } from "@tanstack/react-query";
import { useTranslation, Trans } from "react-i18next";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import { errorMessage } from "@/lib/errorMessage";
import { fileFromDrop } from "@/lib/importDrop";
import { cn } from "@/lib/utils";
import type { ImportArgs, ImportResult } from "@/hooks/snapshotImport";

type Props = {
  // templateUrl + mutation are owned by the parent so the same dialog drives
  // import for any amount-shape position group (asset/liability/receivable).
  templateUrl: string;
  mutation: UseMutationResult<ImportResult, unknown, ImportArgs>;
  currency: string;
};

// Two-step bulk import: download a scoped template, fill it offline, then
// upload. "Check file" runs a server-side dry-run (validates + counts, writes
// nothing); "Import" only lights up once the file is clean. Aimed at a
// non-technical user backfilling years of statements at once.
export function ImportSnapshotsDialog({
  templateUrl,
  mutation,
  currency,
}: Props) {
  const { t } = useTranslation("common");
  const [open, setOpen] = useState(false);
  const [file, setFile] = useState<File | null>(null);
  const [result, setResult] = useState<ImportResult | null>(null);
  const [dragActive, setDragActive] = useState(false);
  const [invalid, setInvalid] = useState(false);
  const fileRef = useRef<HTMLInputElement>(null);

  function reset() {
    setFile(null);
    setResult(null);
    setDragActive(false);
    setInvalid(false);
    mutation.reset();
    if (fileRef.current) fileRef.current.value = "";
  }
  function close() {
    setOpen(false);
    reset();
  }

  // Single funnel for both the picker and the drop zone so a non-.xlsx is
  // rejected identically either way. An `empty` drop leaves the selection as-is.
  function selectFiles(files: FileList | File[] | null) {
    const outcome = fileFromDrop(files);
    if (!outcome.ok && outcome.reason === "empty") return;
    setResult(null);
    mutation.reset();
    if (!outcome.ok) {
      setFile(null);
      setInvalid(true);
      if (fileRef.current) fileRef.current.value = "";
      return;
    }
    setInvalid(false);
    setFile(outcome.file);
  }

  function onPickFile(e: React.ChangeEvent<HTMLInputElement>) {
    selectFiles(e.target.files);
  }

  function onDragOver(e: React.DragEvent) {
    e.preventDefault();
    setDragActive(true);
  }
  function onDragLeave(e: React.DragEvent) {
    e.preventDefault();
    setDragActive(false);
  }
  function onDrop(e: React.DragEvent) {
    e.preventDefault();
    setDragActive(false);
    selectFiles(e.dataTransfer.files);
  }

  function run(mode: "preview" | "commit") {
    if (!file) return;
    mutation.mutate({ file, mode }, { onSuccess: setResult });
  }

  const total = result ? result.to_insert + result.to_update : 0;
  const cleanPreview =
    result?.mode === "preview" && result.errors.length === 0 && total > 0;
  const emptyPreview =
    result?.mode === "preview" && result.errors.length === 0 && total === 0;
  const committed = result?.committed === true;

  return (
    <Dialog open={open} onOpenChange={(o) => (o ? setOpen(true) : close())}>
      <DialogTrigger asChild>
        <Button
          size="sm"
          variant="outline"
          data-testid="import-snapshots-trigger"
        >
          <Upload className="mr-1 size-4" />
          {t("import.trigger")}
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("import.title")}</DialogTitle>
          <DialogDescription>{t("import.description")}</DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          <div className="grid gap-1.5">
            <Label>{t("import.step1Label")}</Label>
            <a
              href={templateUrl}
              className="text-sm text-primary underline underline-offset-4"
              data-testid="import-template-link"
            >
              {t("import.templateLink")}
            </a>
            <p className="text-xs text-muted-foreground">
              {/* <bold> tag in the catalog renders via Trans → <strong>. Keeps
                  the inline emphasis without splitting the sentence into three
                  separate keys per locale. */}
              <Trans
                i18nKey="import.step1Hint"
                t={t}
                values={{ currency }}
                components={{ bold: <strong /> }}
              />
            </p>
          </div>

          <div className="grid gap-1.5">
            <Label htmlFor="import-file">{t("import.step2Label")}</Label>
            <div
              onDragOver={onDragOver}
              onDragLeave={onDragLeave}
              onDrop={onDrop}
              data-testid="import-drop-zone"
              data-drag-active={dragActive}
              className={cn(
                "grid gap-2 rounded-md border-2 border-dashed p-4 transition-colors",
                dragActive
                  ? "border-primary bg-primary/5"
                  : "border-input bg-muted/30",
              )}
            >
              <p className="text-sm text-muted-foreground">
                {dragActive ? t("import.dropActive") : t("import.dropHint")}
              </p>
              <input
                id="import-file"
                ref={fileRef}
                type="file"
                accept=".xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
                onChange={onPickFile}
                className="text-sm"
                data-testid="import-file-input"
              />
            </div>
            {invalid && (
              <p
                className="text-sm text-destructive"
                data-testid="import-invalid-file"
              >
                {t("import.invalidFile")}
              </p>
            )}
            {file && !invalid && (
              <p
                className="text-xs text-muted-foreground"
                data-testid="import-selected-file"
              >
                {file.name}
              </p>
            )}
          </div>

          {result && !committed && (
            <div className="text-sm" data-testid="import-result">
              {result.errors.length > 0 ? (
                <div className="space-y-1">
                  <p className="text-destructive">
                    {t("import.needsFixing", { count: result.errors.length })}
                  </p>
                  <ul
                    className="list-disc space-y-0.5 pl-5 text-destructive"
                    data-testid="import-errors"
                  >
                    {result.errors.map((e) => (
                      <li key={e.row}>
                        {t("import.rowError", {
                          row: e.row,
                          message: e.message,
                        })}
                      </li>
                    ))}
                  </ul>
                </div>
              ) : emptyPreview ? (
                <p className="text-muted-foreground">
                  {t("import.emptyPreview")}
                </p>
              ) : (
                <p className="text-muted-foreground">
                  {t("import.preview", {
                    insert: result.to_insert,
                    update: result.to_update,
                  })}
                </p>
              )}
            </div>
          )}

          {committed && (
            <p className="text-sm text-emerald-600" data-testid="import-done">
              {t("import.committed", {
                count: total,
                insert: result.to_insert,
                update: result.to_update,
              })}
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
              {t("actions.done")}
            </Button>
          ) : (
            <>
              <Button type="button" variant="outline" onClick={close}>
                {t("cancel")}
              </Button>
              <Button
                type="button"
                variant="outline"
                disabled={!file || mutation.isPending}
                onClick={() => run("preview")}
                data-testid="import-check-btn"
              >
                {mutation.isPending && !committed
                  ? t("actions.checking")
                  : t("import.checkFile")}
              </Button>
              <Button
                type="button"
                disabled={!cleanPreview || mutation.isPending}
                onClick={() => run("commit")}
                data-testid="import-commit-btn"
              >
                {cleanPreview
                  ? t("import.importN", { count: total })
                  : t("import.importPlain")}
              </Button>
            </>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
