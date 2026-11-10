import { createContext, useContext } from "react";

// Per-field validation state for a form (#824, ADR-0058 §5). `ValidatedForm`
// provides it; `Input`, `Select` and the month picker read it by their `id`, so
// a field gets `aria-invalid`, `aria-describedby` and its message rendered
// right under it without any per-form wiring. Outside a provider every lookup
// is empty and the primitives behave exactly as before.
export type FormErrors = {
  /** The message to show under the field with this id, if any. */
  messageFor: (id: string | undefined) => string | undefined;
  /** A field changed by something other than a DOM event (e.g. a picker). */
  dismissServerError: () => void;
  /** A server error that belongs to no single field, for <FormError />. */
  formMessage?: string;
};

export const FormErrorsContext = createContext<FormErrors | null>(null);

export function useFormErrors(): FormErrors | null {
  return useContext(FormErrorsContext);
}

export function errorId(id: string): string {
  return `${id}-error`;
}

// What a control needs to announce its error.
export function useFieldError(id: string | undefined) {
  const message = useFormErrors()?.messageFor(id);
  return { message, invalid: message !== undefined };
}

export function FieldError({ id, children }: { id: string; children: string }) {
  return (
    <p id={errorId(id)} data-testid={`field-error-${id}`} className="text-xs text-destructive">
      {children}
    </p>
  );
}

// Describes the field by its error message while one is showing.
export function describedBy(
  existing: string | undefined,
  id: string | undefined,
  invalid: boolean,
): string | undefined {
  if (!invalid || !id) return existing;
  return [existing, errorId(id)].filter(Boolean).join(" ");
}
