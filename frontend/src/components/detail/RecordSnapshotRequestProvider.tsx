import { useCallback, useMemo, useState, type ReactNode } from "react";
import { RecordSnapshotRequestContext } from "@/lib/recordSnapshotRequest";

// One per position detail page (#830): lets a trade's toast and the mismatch banner
// ask that page's Record snapshot dialog to open. See lib/recordSnapshotRequest.
export function RecordSnapshotRequestProvider({ children }: { children: ReactNode }) {
  const [seq, setSeq] = useState(0);
  const request = useCallback(() => setSeq((n) => n + 1), []);
  const value = useMemo(() => ({ seq, available: true, request }), [seq, request]);
  return (
    <RecordSnapshotRequestContext.Provider value={value}>
      {children}
    </RecordSnapshotRequestContext.Provider>
  );
}
