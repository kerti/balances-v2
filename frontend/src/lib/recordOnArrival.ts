import { createContext, useContext } from "react";
import type { NavigateOptions } from "react-router";

// "Land on the new item" (#843). Creating a position navigates to its detail
// page with the Record snapshot dialog already open. The router state carries
// the request across the navigation; `DetailRoute` (App.tsx) reads it once,
// clears it from the history entry (so a refresh or back-navigation never
// re-opens the dialog) and offers it to the Record snapshot dialogs through
// this context, which seed their initial `open` from it.
export const RecordOnArrivalContext = createContext(false);

export function useRecordOnArrival(): boolean {
  return useContext(RecordOnArrivalContext);
}

type ArrivalState = { recordSnapshot?: boolean } | null | undefined;

export function wantsRecordOnArrival(state: unknown): boolean {
  return (state as ArrivalState)?.recordSnapshot === true;
}

// Navigation options for a list's `onSelect(id, { recordSnapshot })`: the
// create dialogs ask for it, a plain row click does not.
export function arrivalOptions(opts?: { recordSnapshot?: boolean }): NavigateOptions | undefined {
  return opts?.recordSnapshot ? { state: { recordSnapshot: true } } : undefined;
}
