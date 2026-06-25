import type { CSSProperties } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { Outlet } from "react-router";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { UserAvatar } from "@/components/UserAvatar";
import { AppSidebar } from "@/components/AppSidebar";
import { AppLogo } from "@/components/AppLogo";
import {
  SidebarInset,
  SidebarProvider,
  SidebarTrigger,
} from "@/components/ui/sidebar";
import { useSession } from "@/hooks/useSession";
import { useInviteIgnoredNotice } from "@/hooks/useInviteIgnoredNotice";
import { useLocaleReconcile } from "@/i18n/useLocaleReconcile";
import { useThemeReconcile } from "@/theme/useThemeReconcile";
import { api } from "@/api/client";

// The authenticated layout: persistent sidebar on desktop, a hamburger-opened
// drawer on phones (handled by SidebarProvider/Sidebar). The routed page renders
// into <Outlet/>. Mounted only when signed in, so useSession always has a user.
export function AppShell() {
  const qc = useQueryClient();
  const { data: user } = useSession();
  const { t } = useTranslation("common");
  useLocaleReconcile(user);
  useThemeReconcile(user);
  useInviteIgnoredNotice(user);

  async function handleSignOut() {
    try {
      await api("/api/auth/logout", { method: "POST" });
    } finally {
      // Whatever happens on the server, surface the signed-out state on the
      // client by invalidating the session query.
      await qc.invalidateQueries({ queryKey: ["session"] });
    }
  }

  if (!user) return null;

  return (
    <SidebarProvider
      // Narrower than shadcn's 16rem default; the longest label ("Institutional")
      // fits comfortably at the reduced font size. Mobile drawer keeps its own
      // wider width (set inside the Sheet branch of the Sidebar component).
      style={{ "--sidebar-width": "12rem" } as CSSProperties}
    >
      <AppSidebar />
      <SidebarInset>
        <header className="sticky top-0 z-10 flex items-center justify-between gap-4 border-b border-border bg-background px-4 py-3 md:px-6">
          <div className="flex items-center gap-2">
            {/* Drawer toggle: phones only — the sidebar is always visible on desktop. */}
            <SidebarTrigger className="md:hidden" />
            <div className="md:hidden">
              <AppLogo />
            </div>
          </div>
          <div className="flex items-center gap-3">
            <div className="flex items-center gap-2">
              <UserAvatar
                name={user.display_name}
                pictureUrl={user.picture_url}
              />
              <div className="hidden text-sm sm:block">
                <div className="text-foreground">{user.display_name}</div>
                <div className="text-xs text-muted-foreground">
                  {user.email}
                </div>
              </div>
            </div>
            <Button variant="outline" size="sm" onClick={handleSignOut}>
              {t("signOut")}
            </Button>
          </div>
        </header>
        <main className="w-full p-4 md:p-6">
          <div className="mx-auto w-full max-w-4xl">
            <Outlet />
          </div>
        </main>
      </SidebarInset>
    </SidebarProvider>
  );
}
