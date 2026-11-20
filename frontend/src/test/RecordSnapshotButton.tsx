import { useRecordSnapshotRequest } from "@/lib/recordSnapshotRequest";

// Stands in for the detail page header's Record snapshot button (#935) when a
// test renders a Record snapshot dialog on its own: the dialogs have no trigger.
export function RecordSnapshotButton() {
  const { open } = useRecordSnapshotRequest();
  return (
    <button type="button" data-testid="snapshot-record" onClick={open}>
      Record snapshot
    </button>
  );
}
