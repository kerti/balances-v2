import { useEffect, useState } from "react";
import { WifiOff } from "lucide-react";
import { useTranslation } from "react-i18next";
import { onConnectivityChange } from "@/api/client";

// navigator.onLine alone is not enough — it lies about captive portals and
// about a server that is simply unreachable — so this also listens to the API
// client's own signal, which is what clears the strip the moment a request
// gets through again.
function useConnectivity(): boolean {
  const [online, setOnline] = useState(() => navigator.onLine);

  useEffect(() => {
    const goOnline = () => setOnline(true);
    const goOffline = () => setOnline(false);
    window.addEventListener("online", goOnline);
    window.addEventListener("offline", goOffline);
    const unsubscribe = onConnectivityChange(setOnline);
    return () => {
      window.removeEventListener("online", goOnline);
      window.removeEventListener("offline", goOffline);
      unsubscribe();
    };
  }, []);

  return online;
}

// The service worker caches the shell only (ADR-0055), so offline the app
// opens but no data loads; this strip says why instead of leaving an empty
// screen that looks broken. Muted, not destructive: losing signal is normal.
export function OfflineBanner() {
  const { t } = useTranslation("common");
  const online = useConnectivity();

  if (online) return null;

  return (
    <div
      role="status"
      data-testid="offline-banner"
      className="flex items-center gap-2 bg-muted px-4 py-2 text-sm text-foreground"
    >
      <WifiOff aria-hidden="true" className="size-4 shrink-0" />
      {t("offline")}
    </div>
  );
}
