import {
  useState,
  type ChangeEvent,
  type ComponentProps,
  type FocusEvent,
  type FormEvent,
} from "react";
import { Callout } from "@/components/common/Callout";
import { FormErrorsContext, useFormErrors } from "@/components/ui/field-error";
import { errorMessage } from "@/lib/errorMessage";
import { fieldForError, idMatchesField, isControl, nativeMessage } from "@/lib/formErrors";

// Field id -> message, for rules the browser's own constraints can't express.
export type FieldMessages = Partial<Record<string, string>>;

type Props = Omit<ComponentProps<"form">, "onSubmit" | "noValidate"> & {
  /** Runs only once every field is valid. */
  onSubmit: (e: FormEvent<HTMLFormElement>) => void;
  /**
   * Extra rules as a pure function of the form's current state. Keyed by the
   * id of the field the message belongs to; shown after that field is left or
   * the form is submitted, and re-evaluated on every render so it clears as
   * the user fixes it.
   */
  validate?: () => FieldMessages;
  /** The mutation error: drawn beside its field when the code maps to one, else by <FormError />. */
  error?: unknown;
};

function withMessage(prev: FieldMessages, id: string, message: string | undefined) {
  const next = { ...prev };
  if (message) next[id] = message;
  else delete next[id];
  return next;
}

// The one place a form dialog validates (#824, ADR-0058 §5). Replaces the
// native browser bubbles (`noValidate`) with a message under every invalid
// field, shown on blur and on submit, all at once on submit. Submit buttons
// stay enabled: clicking one shows what is missing. A server error code that
// belongs to a field (lib/formErrors) renders under it; any other error shows
// as a form-level <FormError /> where the dialog places it.
export function ValidatedForm({ onSubmit, validate, error, children, ...rest }: Props) {
  const [native, setNative] = useState<FieldMessages>({});
  const [touched, setTouched] = useState<Record<string, true>>({});
  const [submitted, setSubmitted] = useState(false);
  // The server error the user has since started to fix, by identity.
  const [dismissed, setDismissed] = useState<unknown>(null);

  const custom = validate?.() ?? {};
  const serverActive = error != null && error !== dismissed;
  const serverField = serverActive ? fieldForError(error) : undefined;
  const serverText = serverActive ? errorMessage(error) : undefined;

  function messageFor(id: string | undefined): string | undefined {
    if (!id) return undefined;
    const client = (submitted || touched[id] ? custom[id] : undefined) ?? native[id];
    if (client) return client;
    if (serverField && serverText && idMatchesField(id, serverField)) return serverText;
    return undefined;
  }

  function dismissServerError() {
    if (error != null) setDismissed(error);
  }

  function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const elements = Array.from(e.currentTarget.elements);
    const next: FieldMessages = {};
    for (const el of elements) {
      if (!isControl(el) || !el.id || !el.willValidate) continue;
      const message = nativeMessage(el);
      if (message) next[el.id] = message;
    }
    setNative(next);
    setSubmitted(true);
    if (Object.keys(next).length > 0 || Object.keys(custom).length > 0) {
      const first = elements.find((el) => el.id && (next[el.id] || custom[el.id]));
      if (first instanceof HTMLElement) first.focus();
      return;
    }
    onSubmit(e);
  }

  function handleBlur(e: FocusEvent<HTMLFormElement>) {
    const el = e.target;
    if (!isControl(el) || !el.id) return;
    // Moving onto a button (Cancel, Submit) must not shift the layout under the
    // pointer before the click lands; Submit validates everything itself.
    if (e.relatedTarget instanceof HTMLButtonElement) return;
    setNative((prev) => withMessage(prev, el.id, nativeMessage(el)));
    setTouched((prev) => ({ ...prev, [el.id]: true }));
  }

  function handleChange(e: ChangeEvent<HTMLFormElement>) {
    const el = e.target;
    dismissServerError();
    if (!isControl(el) || !el.id || !native[el.id]) return;
    setNative((prev) => withMessage(prev, el.id, nativeMessage(el)));
  }

  return (
    <FormErrorsContext.Provider
      value={{
        messageFor,
        dismissServerError,
        formMessage: serverActive && !serverField ? serverText : undefined,
      }}
    >
      <form
        {...rest}
        noValidate
        onSubmit={handleSubmit}
        onBlur={handleBlur}
        onChange={handleChange}
      >
        {children}
      </form>
    </FormErrorsContext.Provider>
  );
}

// The form-level message: a server error that belongs to no single field.
export function FormError() {
  const message = useFormErrors()?.formMessage;
  if (!message) return null;
  return (
    <Callout variant="warning" data-testid="form-error">
      {message}
    </Callout>
  );
}
