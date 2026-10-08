import { createContext, useContext, type ReactNode } from "react";
import { RefreshCw } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useRegisterSW } from "virtual:pwa-register/react";
import { Button } from "@/components/ui/button";

type AppUpdate = { needRefresh: boolean; reload: () => void };

const AppUpdateContext = createContext<AppUpdate>({ needRefresh: false, reload: () => {} });

// The only place the service worker is registered (ADR-0055). Mounted once
// above App so registration survives the switch between the pre-auth screens
// and the signed-in shell, each of which renders its own UpdateBanner.
export function AppUpdateProvider({ children }: { children: ReactNode }) {
  const {
    needRefresh: [needRefresh],
    updateServiceWorker,
  } = useRegisterSW();
  return (
    <AppUpdateContext.Provider
      value={{ needRefresh, reload: () => void updateServiceWorker(true) }}
    >
      {children}
    </AppUpdateContext.Provider>
  );
}

// With registerType 'prompt' a new build installs in the background and then
// waits; nothing on screen changes until the user taps Reload, so a
// half-filled dialog survives a deploy. Calm secondary tokens: a waiting
// update is good news, not a problem.
export function UpdateBanner() {
  const { t } = useTranslation("common");
  const { needRefresh, reload } = useContext(AppUpdateContext);

  if (!needRefresh) return null;

  return (
    <div
      role="status"
      data-testid="update-banner"
      className="flex items-center justify-between gap-3 bg-secondary px-4 py-2 text-sm text-secondary-foreground"
    >
      <span className="flex items-center gap-2">
        <RefreshCw aria-hidden="true" className="size-4 shrink-0" />
        {t("appUpdate.available")}
      </span>
      <Button size="sm" onClick={reload}>
        {t("appUpdate.reload")}
      </Button>
    </div>
  );
}
