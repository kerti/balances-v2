// "Update {Mon} holding →" (#830). A trade's toast and the quantity-mismatch banner
// both offer to open the position's Record snapshot dialog, prefilled. The dialog
// lives in the snapshot section while the toast fires from a mutation hook, so the
// two meet through this per-position-page context: a caller asks, the dialog (one
// per page) opens itself. Asking only opens a dialog - nothing is written until the
// user presses Save in it (INV-SNAPSHOTS-18).
//
// Outside a provider (a list screen, a test that renders a hook alone) `request`
// is a no-op and `available` is false, so callers can fall back to a plain toast.
// The provider component lives in components/detail/RecordSnapshotRequestProvider.
import { createContext, useContext, useState } from "react";

export type RecordSnapshotRequest = {
  // Bumps once per request; a dialog compares it with the last value it saw.
  seq: number;
  available: boolean;
  request: () => void;
};

export const RecordSnapshotRequestContext = createContext<RecordSnapshotRequest>({
  seq: 0,
  available: false,
  request: () => {},
});

export function useRecordSnapshotRequest(): RecordSnapshotRequest {
  return useContext(RecordSnapshotRequestContext);
}

// For the Record snapshot dialogs: runs `onRequest` once for each new request.
// State is adjusted during render (React's "derive from props" pattern) rather
// than in an effect, so there is no flash of the closed dialog and no
// setState-in-effect.
export function useOnRecordRequest(onRequest: () => void) {
  const { seq } = useContext(RecordSnapshotRequestContext);
  const [seen, setSeen] = useState(seq);
  if (seq !== seen) {
    setSeen(seq);
    onRequest();
  }
}
