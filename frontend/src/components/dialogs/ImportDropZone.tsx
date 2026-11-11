import { useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { buttonVariants } from "@/components/ui/button";
import { cn } from "@/lib/utils";

type Props = {
  inputId: string;
  // Receives whatever the picker or the drop carried; the caller validates it
  // (lib/importDrop.fileFromDrop) so both paths reject a non-.xlsx the same way.
  onFiles: (files: FileList | File[] | null) => void;
};

// The importers' file target. The native "Choose File / No file chosen" input
// is visually hidden but stays in the tab order and the accessibility tree; a
// styled label next to it opens the picker and takes the focus ring. The caller
// shows the chosen filename once, so the input's own readout is never needed.
export function ImportDropZone({ inputId, onFiles }: Props) {
  const { t } = useTranslation("common");
  const [dragActive, setDragActive] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);

  function onPick(e: React.ChangeEvent<HTMLInputElement>) {
    onFiles(e.target.files);
    // The caller already holds the File, so clear the input: picking the same
    // file again (after editing it) still fires a change and re-runs the check.
    if (inputRef.current) inputRef.current.value = "";
  }

  return (
    <div
      onDragOver={(e) => {
        e.preventDefault();
        setDragActive(true);
      }}
      onDragLeave={(e) => {
        e.preventDefault();
        setDragActive(false);
      }}
      onDrop={(e) => {
        e.preventDefault();
        setDragActive(false);
        onFiles(e.dataTransfer.files);
      }}
      data-testid="import-drop-zone"
      data-drag-active={dragActive}
      className={cn(
        "flex flex-wrap items-center gap-2 rounded-md border-2 border-dashed p-4 transition-colors",
        dragActive ? "border-primary bg-primary/5" : "border-input bg-muted/30",
      )}
    >
      <p className="text-sm text-muted-foreground">
        {dragActive ? t("import.dropActive") : t("import.dropHint")}
      </p>
      <input
        id={inputId}
        ref={inputRef}
        type="file"
        accept=".xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
        onChange={onPick}
        className="peer sr-only"
        data-testid="import-file-input"
      />
      <label
        htmlFor={inputId}
        className={cn(
          buttonVariants({ variant: "outline", size: "sm" }),
          "cursor-pointer peer-focus-visible:border-ring peer-focus-visible:ring-[3px] peer-focus-visible:ring-ring/50",
        )}
      >
        {t("import.chooseFile")}
      </label>
    </div>
  );
}
