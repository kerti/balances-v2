import { useState, type KeyboardEvent } from "react";
import { toastSaved } from "@/lib/writeToast";

// The one shape every typed Settings value shares (ADR-0058 §7, #823): an
// explicit Save that is enabled only while the field is dirty and valid, Enter
// to save, Esc to revert to the stored value, and one toast when the write
// lands. A half-typed currency code or an inflation typo must never reach the
// server by itself, which is why these are not autosaved like the choices.
//
// `draft === null` means "untouched, show the stored value"; once the user
// types, the draft (even "") wins. `normalize` is what is compared and written
// (trim, upper-case); `isValid` gates Save on the normalized value. `write`
// starts the mutation and must call `done` on success - the hook then clears
// the draft and raises the toast. A failure is the row's own inline error.
export function useTypedSetting({
  stored,
  normalize = (v) => v.trim(),
  isValid = () => true,
  pending,
  write,
}: {
  stored: string;
  normalize?: (value: string) => string;
  isValid?: (normalized: string) => boolean;
  pending: boolean;
  write: (normalized: string, done: () => void) => void;
}) {
  const [draft, setDraft] = useState<string | null>(null);

  const value = draft ?? stored;
  const normalized = normalize(value);
  const dirty = normalized !== normalize(stored);
  const canSave = dirty && isValid(normalized) && !pending;

  const save = () => {
    if (!canSave) return;
    write(normalized, () => {
      setDraft(null);
      toastSaved();
    });
  };
  const revert = () => setDraft(null);

  const onKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter") {
      e.preventDefault();
      save();
    } else if (e.key === "Escape" && draft !== null) {
      e.preventDefault();
      revert();
    }
  };

  return { value, setValue: setDraft, canSave, save, onKeyDown };
}
