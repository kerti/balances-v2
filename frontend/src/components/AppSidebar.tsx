import { Link, useLocation } from 'react-router'
import { useTranslation } from 'react-i18next'
import { CircleUser } from 'lucide-react'
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
} from '@/components/ui/sidebar'
import { routes } from '@/lib/routes'
import { AppLogo } from '@/components/AppLogo'
import { GitHubMark } from '@/components/icons/GitHubMark'
import {
  APP_VERSION,
  DEPLOY_ENV,
  MAINTAINER_URL,
  REPO_URL,
} from '@/lib/appInfo'

// `labelKey` indexes into the `nav` namespace catalog rather than carrying the
// EN string inline — keeps the structural NAV array translation-agnostic.
type Leaf = { labelKey: string; to: string }
// A top-level destination. With `children` it's a group: the button links to
// the group home and the subtype lists render beneath it (always expanded — few
// enough items that hiding them behind a collapse would only add a click).
type Section = { labelKey: string; to: string; children?: Leaf[] }

const NAV: Section[] = [
  { labelKey: 'dashboard', to: routes.dashboard },
  {
    labelKey: 'assets',
    to: routes.assets,
    children: [
      { labelKey: 'bankAccounts', to: routes.bankAccounts },
      { labelKey: 'properties', to: routes.properties },
      { labelKey: 'vehicles', to: routes.vehicles },
    ],
  },
  {
    labelKey: 'liabilities',
    to: routes.liabilities,
    children: [
      { labelKey: 'personal', to: routes.liabilitiesPersonal },
      { labelKey: 'institutional', to: routes.liabilitiesInstitutional },
    ],
  },
  { labelKey: 'receivables', to: routes.receivables },
  {
    labelKey: 'investments',
    to: routes.investments,
    children: [
      { labelKey: 'stocks', to: routes.stocks },
      { labelKey: 'mutualFunds', to: routes.mutualFunds },
      { labelKey: 'bonds', to: routes.bonds },
      { labelKey: 'timeDeposits', to: routes.timeDeposits },
      { labelKey: 'gold', to: routes.gold },
    ],
  },
  { labelKey: 'income', to: routes.income },
  { labelKey: 'tags', to: routes.tags },
  { labelKey: 'settings', to: routes.settings },
]

// Uses shadcn's default text-sm for both main and sub items so the menu reads at
// a normal size; the active item uses the accent fill (set explicitly so the
// active style is legible here rather than inherited from the cva).
const navItemClass =
  'data-[active=true]:bg-sidebar-accent data-[active=true]:text-sidebar-accent-foreground'

export function AppSidebar() {
  const { pathname } = useLocation()
  const { setOpenMobile } = useSidebar()
  const { t } = useTranslation(['nav', 'common'])
  // Close the mobile drawer after a navigation; a no-op on desktop.
  const close = () => setOpenMobile(false)

  // A leaf/childless destination stays highlighted while you're on it or any
  // detail page beneath it (e.g. Bank Accounts active on /assets/bank-accounts
  // and /assets/bank-accounts/:id). The dashboard's `/` is exact-only — the
  // prefix test below reduces to an equality check for it.
  const leafActive = (to: string) =>
    pathname === to || pathname.startsWith(to + '/')

  return (
    <Sidebar>
      <SidebarHeader>
        <div className="px-2 py-1">
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
                    isActive={
                      section.children
                        ? pathname === section.to
                        : leafActive(section.to)
                    }
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
      </SidebarContent>
      <SidebarFooter>
        <div className="flex flex-col gap-2 px-2 py-1 text-xs text-muted-foreground">
          <div className="flex items-center justify-between gap-2">
            <span data-testid="app-version" className="font-mono">
              {APP_VERSION}
            </span>
            <span
              data-testid="deploy-env"
              className="rounded border border-sidebar-border px-1.5 py-0.5 text-[10px] uppercase tracking-wide"
            >
              {t(`footer.deploy.${DEPLOY_ENV}`)}
            </span>
          </div>
          <div className="flex flex-col gap-1.5">
            <a
              href={REPO_URL}
              target="_blank"
              rel="noreferrer noopener"
              aria-label={t('footer.sourceCode')}
              title={t('footer.sourceCode')}
              data-testid="footer-link-github"
              className="flex w-fit items-center gap-1.5 transition-colors hover:text-foreground"
            >
              <GitHubMark className="h-4 w-4" />
              {t('footer.sourceCodeLabel')}
            </a>
            <a
              href={MAINTAINER_URL}
              target="_blank"
              rel="noreferrer noopener"
              aria-label={t('footer.website')}
              title={t('footer.website')}
              data-testid="footer-link-website"
              className="flex w-fit items-center gap-1.5 transition-colors hover:text-foreground"
            >
              <CircleUser className="h-4 w-4" />
              {t('footer.maintainerLabel')}
            </a>
          </div>
        </div>
      </SidebarFooter>
    </Sidebar>
  )
}
