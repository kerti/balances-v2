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
// (issue #123) so the two never drift.
//
// Two layouts (issue #131):
//  - "stacked" (default, sidebar footer): the rail is narrow (12rem), so each
//    item gets its own line — version above the deploy chip, links below.
//  - "split" (sign-in card): wider, so version (left) pairs with the deploy
//    chip (right) on one row, and the two links pair left/right on the next.
type Props = {
  className?: string
  variant?: 'stacked' | 'split'
}

export function AppInfo({ className, variant = 'stacked' }: Props) {
  const { t } = useTranslation('nav')

  const version = (
    <span data-testid="app-version" className="font-mono">
      {APP_VERSION}
    </span>
  )
  const chip = (
    <span
      data-testid="deploy-env"
      className="w-fit rounded border border-sidebar-border px-1.5 py-0.5 text-[10px] uppercase tracking-wide"
    >
      {t(`footer.deploy.${DEPLOY_ENV}`)}
    </span>
  )
  const github = (
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
  )
  const maintainer = (
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
  )

  const base = `flex flex-col gap-2 text-xs text-muted-foreground${
    className ? ` ${className}` : ''
  }`

  if (variant === 'split') {
    return (
      <div className={`w-full ${base}`}>
        <div className="flex items-center justify-between gap-2">
          {version}
          {chip}
        </div>
        <div className="flex items-center justify-between gap-2">
          {github}
          {maintainer}
        </div>
      </div>
    )
  }

  return (
    <div className={base}>
      <div className="flex flex-col gap-1">
        {version}
        {chip}
      </div>
      <div className="flex flex-col gap-1.5">
        {github}
        {maintainer}
      </div>
    </div>
  )
}
