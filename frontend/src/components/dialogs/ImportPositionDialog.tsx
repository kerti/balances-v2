import { useRef, useState } from "react";
import { Upload } from "lucide-react";
import type { UseMutationResult } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { Callout } from "@/components/common/Callout";
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
import { formatDate, formatDateTime } from "@/lib/format";
import { isActiveStatus } from "@/lib/lifecycle";
import { maskIdentifier } from "@/lib/maskIdentifier";
import { positionDetail } from "@/lib/routes";
import type { CreateImportArgs, CreateImportResult } from "@/hooks/snapshotImport";

// Past this many rows, the list scrolls in its own boundary and the rest
// collapse into a "N more errors" line instead of stretching the dialog
// past the footer (#132).
const MAX_VISIBLE_ERRORS = 8;

type Props = {
  // noun + mutation are owned by the parent so the same dialog can drive
  // create-from-file import for any position group (bank account first).
  noun: string;
  mutation: UseMutationResult<CreateImportResult, unknown, CreateImportArgs>;
};

// Create a brand-new position from a workbook: its Detail sheet becomes the
// position, its Snapshots sheet seeds the history. Picking or dropping a file
// runs a server-side dry-run straight away (validates Detail fields + snapshot
// rows, writes nothing); "Create" is the only action and lights up once the
// file is clean. The format is a position export — the dialog tells the user to
// export an existing one to get it.
export function ImportPositionDialog({ noun, mutation }: Props) {
  const { t } = useTranslation("common");
  const [open, setOpen] = useState(false);
  const [file, setFile] = useState<File | null>(null);
  const [result, setResult] = useState<CreateImportResult | null>(null);
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

  const fieldErrors = result?.field_errors ?? [];
  const rowErrors = result?.errors ?? [];
  const hasErrors = fieldErrors.length > 0 || rowErrors.length > 0;
  const errorCount = fieldErrors.length + rowErrors.length;
  const visibleFieldErrors = fieldErrors.slice(0, MAX_VISIBLE_ERRORS);
  const visibleRowErrors = rowErrors.slice(
    0,
    Math.max(0, MAX_VISIBLE_ERRORS - visibleFieldErrors.length),
  );
  const cleanPreview = result?.mode === "preview" && result.would_create;
  const committed = result?.committed === true;
  const checking = mutation.isPending && mutation.variables?.mode === "preview";
  // Existing positions this workbook would duplicate (#828). Never blocking: a
  // duplicate can be legitimate, so a match only re-labels Create as secondary.
  const matches = cleanPreview ? (result?.matches ?? []) : [];
  const hasMatch = matches.length > 0;
  // A date alone can't tell apart two copies made the same day; add the time then.
  const matchDays = matches.map((m) => formatDate(m.created_at));
  const addedOn = (i: number) =>
    matchDays.filter((d) => d === matchDays[i]).length > 1
      ? formatDateTime(matches[i].created_at)
      : matchDays[i];
  const createLabel = hasMatch ? t("importCreate.createAnywayBtn") : t("importCreate.createBtn");

  return (
    <Dialog open={open} onOpenChange={(o) => (o ? setOpen(true) : close())}>
      <DialogTrigger asChild>
        <Button variant="outline" data-testid="import-position-trigger">
          <Upload className="mr-1 size-4" />
          {t("importCreate.trigger")}
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("importCreate.title", { noun })}</DialogTitle>
          <DialogDescription>{t("importCreate.description", { noun })}</DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          <div className="grid gap-1.5">
            <Label htmlFor="import-create-file">{t("importCreate.uploadLabel")}</Label>
            <ImportDropZone inputId="import-create-file" onFiles={selectFiles} />
            <p className="text-xs text-muted-foreground">
              {t("importCreate.formatHint", { noun })}
            </p>
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
              {hasErrors ? (
                <div className="space-y-1">
                  <p className="text-destructive">
                    {t("importCreate.needsFixing", {
                      count: errorCount,
                    })}
                  </p>
                  <ul
                    className="max-h-48 list-disc space-y-0.5 overflow-y-auto pl-5 text-destructive"
                    data-testid="import-errors"
                  >
                    {visibleFieldErrors.map((e) => (
                      <li key={`f-${e.field}`}>
                        {t("importCreate.fieldError", {
                          field: e.field,
                          message: e.message,
                        })}
                      </li>
                    ))}
                    {visibleRowErrors.map((e) => (
                      <li key={`r-${e.row}`}>
                        {t("import.rowError", {
                          row: e.row,
                          message: e.message,
                        })}
                      </li>
                    ))}
                  </ul>
                  {errorCount > MAX_VISIBLE_ERRORS && (
                    <p className="text-xs text-muted-foreground" data-testid="import-errors-more">
                      {t("import.moreErrors", {
                        count: errorCount - MAX_VISIBLE_ERRORS,
                      })}
                    </p>
                  )}
                </div>
              ) : (
                <div className="space-y-2">
                  <p className="text-muted-foreground" data-testid="import-preview">
                    {result.name
                      ? t("importCreate.previewNamed", {
                          noun,
                          count: result.to_insert,
                          who: result.identifier
                            ? `${result.name} (${maskIdentifier(result.identifier)})`
                            : result.name,
                          button: createLabel,
                        })
                      : t("importCreate.preview", { noun, count: result.to_insert })}
                  </p>
                  {hasMatch && (
                    <Callout variant="warning" data-testid="import-match-notice">
                      <p>
                        {t(
                          result.identifier
                            ? "importCreate.matchNameNumber"
                            : "importCreate.matchName",
                          { noun, count: matches.length },
                        )}
                      </p>
                      <ul className="my-1.5 list-disc space-y-1 pl-5">
                        {matches.map((m, i) => {
                          const href = positionDetail(m.group, m.subtype, m.id);
                          const label = result.identifier
                            ? `${m.name} (${maskIdentifier(result.identifier)})`
                            : m.name;
                          return (
                            <li key={m.id} data-testid="import-match-item">
                              {href ? (
                                <Link
                                  to={href}
                                  className="font-medium underline underline-offset-2"
                                  data-testid="import-match-link"
                                >
                                  {label}
                                </Link>
                              ) : (
                                <span className="font-medium">{label}</span>
                              )}
                              {isActiveStatus(m.status) ? "" : t("importCreate.matchClosed")}
                              {t("importCreate.matchAdded", { date: addedOn(i) })}
                            </li>
                          );
                        })}
                      </ul>
                      <p>{t("importCreate.matchHint", { count: matches.length })}</p>
                    </Callout>
                  )}
                </div>
              )}
            </div>
          )}

          {committed && (
            <p className="text-sm text-positive" data-testid="import-done">
              {t("importCreate.committed", { noun, count: result.to_insert })}
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
                variant={hasMatch ? "secondary" : "default"}
                disabled={!cleanPreview || mutation.isPending}
                onClick={commit}
                data-testid="import-commit-btn"
              >
                {createLabel}
              </Button>
            </>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
