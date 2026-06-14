import { useTranslation } from 'react-i18next'
import { CircleUser } from 'lucide-react'
import { GitHubMark } from '@/components/icons/GitHubMark'
import {
  APP_VERSION,
  DEPLOY_ENV,
  MAINTAINER_URL,
  REPO_URL,
} from '@/lib/appInfo'

// App identity block: release tag, deploy target, and the source/maintainer
// links (issue #75). Shared by the sidebar footer and the sign-in screen
// (issue #123) so the two never drift. The version sits on its own line above
// the deploy chip — side-by-side they crowded each other once the tag grew to
// e.g. "v0.6.0-alpha.2".
export function AppInfo({ className }: { className?: string }) {
  const { t } = useTranslation('nav')
  return (
    <div
      className={`flex flex-col gap-2 text-xs text-muted-foreground${
        className ? ` ${className}` : ''
      }`}
    >
      <div className="flex flex-col gap-1">
        <span data-testid="app-version" className="font-mono">
          {APP_VERSION}
        </span>
        <span
          data-testid="deploy-env"
          className="w-fit rounded border border-sidebar-border px-1.5 py-0.5 text-[10px] uppercase tracking-wide"
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
  )
}
