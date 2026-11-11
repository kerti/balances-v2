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
import { ImportDropZone } from "@/components/dialogs/ImportDropZone";
import { errorMessage } from "@/lib/errorMessage";
import { fileFromDrop } from "@/lib/importDrop";
import type { ImportArgs, ImportResult } from "@/hooks/snapshotImport";

// Past this many rows, the list scrolls in its own boundary and the rest
// collapse into a "N more errors" line instead of stretching the dialog
// past the footer (#132).
const MAX_VISIBLE_ERRORS = 8;

type Props = {
  // templateUrl + mutation are owned by the parent so the same dialog drives
  // import for any amount-shape position group (asset/liability/receivable).
  templateUrl: string;
  mutation: UseMutationResult<ImportResult, unknown, ImportArgs>;
  currency: string;
};

// Bulk import: download a scoped template, fill it offline, then upload.
// Picking or dropping a file runs a server-side dry-run straight away
// (validates + counts new / replaced / unchanged months, writes nothing);
// "Import" is the only action and lights up once the file is clean and has
// something to write. Aimed at a non-technical user backfilling years of
// statements at once.
export function ImportSnapshotsDialog({ templateUrl, mutation, currency }: Props) {
  const { t } = useTranslation("common");
  const [open, setOpen] = useState(false);
  const [file, setFile] = useState<File | null>(null);
  const [result, setResult] = useState<ImportResult | null>(null);
  const [invalid, setInvalid] = useState(false);
  // Bumped on every pick and on close so a slow preview for a file the user has
  // since replaced can't land on the newer one.
  const pickRef = useRef(0);

  function reset() {
    pickRef.current++;
    setFile(null);
    setResult(null);
    setInvalid(false);
    mutation.reset();
  }
  function close() {
    setOpen(false);
    reset();
  }

  // Single funnel for both the picker and the drop zone so a non-.xlsx is
  // rejected identically either way, and a good file is checked at once. An
  // `empty` drop leaves the selection as-is.
  function selectFiles(files: FileList | File[] | null) {
    const outcome = fileFromDrop(files);
    if (!outcome.ok && outcome.reason === "empty") return;
    const pick = ++pickRef.current;
    setResult(null);
    mutation.reset();
    if (!outcome.ok) {
      setFile(null);
      setInvalid(true);
      return;
    }
    setInvalid(false);
    setFile(outcome.file);
    mutation.mutate(
      { file: outcome.file, mode: "preview" },
      {
        onSuccess: (r) => {
          if (pick === pickRef.current) setResult(r);
        },
      },
    );
  }

  function commit() {
    if (!file) return;
    mutation.mutate({ file, mode: "commit" }, { onSuccess: setResult });
  }

  const unchanged = result?.unchanged ?? 0;
  const toWrite = result ? result.to_insert + result.to_update : 0;
  const cleanFile = result?.mode === "preview" && result.errors.length === 0;
  const cleanPreview = cleanFile && toWrite > 0;
  const allUnchanged = cleanFile && toWrite === 0 && unchanged > 0;
  const emptyPreview = cleanFile && toWrite === 0 && unchanged === 0;
  const committed = result?.committed === true;
  const checking = mutation.isPending && mutation.variables?.mode === "preview";
  // Only the non-zero counts: "1 replaced, 26 unchanged", not "0 new months, …".
  const counts = (r: ImportResult) => ({
    counts: [
      r.to_insert > 0 && t("import.countNew", { count: r.to_insert }),
      r.to_update > 0 && t("import.countReplaced", { count: r.to_update }),
      (r.unchanged ?? 0) > 0 && t("import.countUnchanged", { count: r.unchanged ?? 0 }),
    ]
      .filter(Boolean)
      .join(", "),
  });

  return (
    <Dialog open={open} onOpenChange={(o) => (o ? setOpen(true) : close())}>
      <DialogTrigger asChild>
        <Button size="sm" variant="outline" data-testid="import-snapshots-trigger">
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
            <ImportDropZone inputId="import-file" onFiles={selectFiles} />
            {invalid && (
              <p className="text-sm text-destructive" data-testid="import-invalid-file">
                {t("import.invalidFile")}
              </p>
            )}
            {file && !invalid && (
              <p className="text-xs text-muted-foreground" data-testid="import-selected-file">
                {file.name}
              </p>
            )}
          </div>

          {checking && (
            <p className="text-sm text-muted-foreground" data-testid="import-checking">
              {t("actions.checking")}
            </p>
          )}

          {result && !committed && (
            <div className="text-sm" data-testid="import-result">
              {result.errors.length > 0 ? (
                <div className="space-y-1">
                  <p className="text-destructive">
                    {t("import.needsFixing", { count: result.errors.length })}
                  </p>
                  <ul
                    className="max-h-48 list-disc space-y-0.5 overflow-y-auto pl-5 text-destructive"
                    data-testid="import-errors"
                  >
                    {result.errors.slice(0, MAX_VISIBLE_ERRORS).map((e) => (
                      <li key={e.row}>
                        {t("import.rowError", {
                          row: e.row,
                          message: e.message,
                        })}
                      </li>
                    ))}
                  </ul>
                  {result.errors.length > MAX_VISIBLE_ERRORS && (
                    <p className="text-xs text-muted-foreground" data-testid="import-errors-more">
                      {t("import.moreErrors", {
                        count: result.errors.length - MAX_VISIBLE_ERRORS,
                      })}
                    </p>
                  )}
                </div>
              ) : emptyPreview ? (
                <p className="text-muted-foreground">{t("import.emptyPreview")}</p>
              ) : allUnchanged ? (
                <p className="text-muted-foreground" data-testid="import-all-unchanged">
                  {t("import.allUnchanged")}
                </p>
              ) : (
                <p className="text-muted-foreground" data-testid="import-preview">
                  {t("import.preview", counts(result))}
                </p>
              )}
            </div>
          )}

          {committed && (
            <p className="text-sm text-positive" data-testid="import-done">
              {t("import.committed", counts(result))}
            </p>
          )}

          {mutation.isError && (
            <p className="text-sm text-destructive">{errorMessage(mutation.error)}</p>
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
                disabled={!cleanPreview || mutation.isPending}
                onClick={commit}
                data-testid="import-commit-btn"
              >
                {t("import.importBtn")}
              </Button>
            </>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
