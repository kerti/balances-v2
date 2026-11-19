import type { FieldMessages } from "@/components/common/ValidatedForm";
import type { EntryType } from "@/api/types";

/** The id of the first radio — the key a `validate` rule uses and submit focuses. */
export function entryTypeFieldId(idPrefix: string): string {
  return `${idPrefix}_entry_type`;
}

/** The Create dialogs' `validate` rule: the question must be answered. */
export function entryTypeRequired(
  idPrefix: string,
  value: EntryType | null,
  message: string,
): FieldMessages {
  return value === null ? { [entryTypeFieldId(idPrefix)]: message } : {};
}
