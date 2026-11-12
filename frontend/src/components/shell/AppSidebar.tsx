import { Link, useLocation } from "react-router";
import { useTranslation } from "react-i18next";
import { X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { UserAvatar } from "@/components/shell/UserAvatar";
import { useSession } from "@/hooks/useSession";
import { useSignOut } from "@/hooks/useSignOut";
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarMenuSub,
  SidebarMenuSubButton,
  SidebarMenuSubItem,
  useSidebar,
} from "@/components/ui/sidebar";
import { routes } from "@/lib/routes";
import { AppLogo } from "@/components/shell/AppLogo";
import { AppInfo } from "@/components/shell/AppInfo";

// `labelKey` indexes into the `nav` namespace catalog rather than carrying the
// EN string inline — keeps the structural NAV array translation-agnostic.
type Leaf = { labelKey: string; to: string };
// A top-level destination. With `children` it's a group: the button links to
// the group home and the subtype lists render beneath it (always expanded — few
// enough items that hiding them behind a collapse would only add a click).
type Section = { labelKey: string; to: string; children?: Leaf[] };

const NAV: Section[] = [
  { labelKey: "dashboard", to: routes.dashboard },
  {
    labelKey: "assets",
    to: routes.assets,
    children: [
      { labelKey: "bankAccounts", to: routes.bankAccounts },
      { labelKey: "properties", to: routes.properties },
      { labelKey: "vehicles", to: routes.vehicles },
    ],
  },
  {
    labelKey: "liabilities",
    to: routes.liabilities,
    children: [
      { labelKey: "personal", to: routes.liabilitiesPersonal },
      { labelKey: "institutional", to: routes.liabilitiesInstitutional },
    ],
  },
  { labelKey: "receivables", to: routes.receivables },
  {
    labelKey: "investments",
    to: routes.investments,
    children: [
      { labelKey: "stocks", to: routes.stocks },
      { labelKey: "mutualFunds", to: routes.mutualFunds },
      { labelKey: "bonds", to: routes.bonds },
      { labelKey: "timeDeposits", to: routes.timeDeposits },
      { labelKey: "gold", to: routes.gold },
    ],
  },
  { labelKey: "income", to: routes.income },
  { labelKey: "tags", to: routes.tags },
  { labelKey: "settings", to: routes.settings },
];

// Uses shadcn's default text-sm for both main and sub items so the menu reads at
// a normal size; the active item uses the accent fill (set explicitly so the
// active style is legible here rather than inherited from the cva).
// In the off-canvas drawer (<1024px) every link clears the 44px tap floor (#559).
const navItemClass =
  "data-[active=true]:bg-sidebar-accent data-[active=true]:text-sidebar-accent-foreground max-lg:h-11";

// The drawer's account section (<1024px): who is signed in, and Sign out, which
// leaves the header at this width.
function DrawerAccount() {
  const { data: user } = useSession();
  const { t } = useTranslation("common");
  const signOut = useSignOut();
  if (!user) return null;
  return (
    <div
      className="flex flex-col gap-2 border-b border-sidebar-border px-2 pb-3"
      data-testid="drawer-account"
    >
      <div className="flex items-center gap-2">
        <UserAvatar name={user.display_name} pictureUrl={user.picture_url} />
        <div className="min-w-0 text-sm">
          <div className="truncate text-foreground" data-testid="drawer-account-name">
            {user.display_name}
          </div>
          <div
            className="truncate text-xs text-muted-foreground"
            data-testid="drawer-account-email"
          >
            {user.email}
          </div>
        </div>
      </div>
      <Button
        variant="outline"
        className="h-11 w-full"
        onClick={signOut}
        data-testid="drawer-sign-out"
      >
        {t("signOut")}
      </Button>
    </div>
  );
}

export function AppSidebar() {
  const { pathname } = useLocation();
  const { isMobile, setOpenMobile } = useSidebar();
  const { t } = useTranslation(["nav", "common"]);
  // Close the mobile drawer after a navigation; a no-op on desktop.
  const close = () => setOpenMobile(false);

  // A leaf/childless destination stays highlighted while you're on it or any
  // detail page beneath it (e.g. Bank Accounts active on /assets/bank-accounts
  // and /assets/bank-accounts/:id). The dashboard's `/` is exact-only — the
  // prefix test below reduces to an equality check for it.
  const leafActive = (to: string) => pathname === to || pathname.startsWith(to + "/");

  return (
    <Sidebar>
      <SidebarHeader>
        {isMobile && (
          <div className="flex justify-end">
            <Button
              variant="ghost"
              size="icon"
              className="size-11"
              onClick={close}
              aria-label={t("closeMenu")}
              data-testid="drawer-close"
            >
              <X />
            </Button>
          </div>
        )}
        {/* Docked only — below 1024px the top header already shows the logo, so
            the drawer would double it up (issue #123). */}
        <div className="hidden px-2 py-1 lg:block">
          <AppLogo className="w-full h-auto" />
        </div>
      </SidebarHeader>
      <SidebarContent>
        <SidebarGroup>
          <SidebarGroupContent>
            <SidebarMenu>
              {NAV.map((section) => (
                <SidebarMenuItem key={section.to}>
                  <SidebarMenuButton
                    asChild
                    className={navItemClass}
                    // A group's own button highlights only on its exact home
                    // path; its children own their own active state.
                    isActive={section.children ? pathname === section.to : leafActive(section.to)}
                  >
                    <Link to={section.to} onClick={close}>
                      {t(section.labelKey)}
                    </Link>
                  </SidebarMenuButton>
                  {section.children && (
                    <SidebarMenuSub>
                      {section.children.map((child) => (
                        <SidebarMenuSubItem key={child.to}>
                          <SidebarMenuSubButton
                            asChild
                            className={navItemClass}
                            isActive={leafActive(child.to)}
                          >
                            <Link to={child.to} onClick={close}>
                              {t(child.labelKey)}
                            </Link>
                          </SidebarMenuSubButton>
                        </SidebarMenuSubItem>
                      ))}
                    </SidebarMenuSub>
                  )}
                </SidebarMenuItem>
              ))}
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>
        {/* In the drawer the app info scrolls at the end of the menu, so only
            the account block is pinned and the menu keeps most of the height. */}
        {isMobile && (
          <AppInfo variant="split" className="mt-auto border-t border-sidebar-border px-4 py-2" />
        )}
      </SidebarContent>
      <SidebarFooter className="shrink-0 border-t border-sidebar-border pt-3">
        {isMobile ? <DrawerAccount /> : <AppInfo className="px-2 py-1" />}
      </SidebarFooter>
    </Sidebar>
  );
}
