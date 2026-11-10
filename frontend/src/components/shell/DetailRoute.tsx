import { useEffect, useState, type ReactNode } from "react";
import { useLocation, useNavigate, useParams, type NavigateFunction } from "react-router";
import { RecordOnArrivalContext, wantsRecordOnArrival } from "@/lib/recordOnArrival";

// Bridges the detail pages' `assetId` / `onBack()` props to the router (App.tsx
// keeps the router out of the ~20 screen components) and consumes the one-shot
// "open Record snapshot" request a create flow leaves in the router state
// (#843): read once on arrival, then stripped from the history entry so a
// refresh or a back-navigation to it never re-opens the dialog.
export function DetailRoute({
  render,
}: {
  render: (id: string, nav: NavigateFunction) => ReactNode;
}) {
  const { id } = useParams();
  const nav = useNavigate();
  const location = useLocation();
  const [recordOnArrival] = useState(() => wantsRecordOnArrival(location.state));
  useEffect(() => {
    if (wantsRecordOnArrival(location.state)) {
      nav(`${location.pathname}${location.search}`, { replace: true, state: null });
    }
  }, [location.state, location.pathname, location.search, nav]);
  return (
    <RecordOnArrivalContext.Provider value={recordOnArrival}>
      {render(id!, nav)}
    </RecordOnArrivalContext.Provider>
  );
}
