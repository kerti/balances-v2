import { useRouteError } from 'react-router'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { clearReloadGuard, isChunkLoadError } from '@/lib/lazyWithReload'

// Terminal, audience-friendly fallback for any error that bubbles out of a
// route — including a chunk-load failure whose one-shot auto-reload
// (lazyWithReload) was already spent this session. Replaces React Router's raw
// developer error dump, which is hostile to a non-technical household.
export function RouteErrorBoundary() {
  const error = useRouteError()
  const { t } = useTranslation(['errors', 'common'])

  const handleReload = () => {
    // Reaching here for a chunk error means the auto-reload guard is set. Clear
    // it so this explicit retry gets a fresh one-shot — the deploy may have
    // settled since the first failure.
    if (isChunkLoadError(error)) clearReloadGuard()
    window.location.reload()
  }

  return (
    <div className="flex min-h-screen flex-col items-center justify-center gap-4 p-6 text-center">
      <h1 className="text-lg font-medium">{t('common:somethingWentWrong')}</h1>
      <p className="max-w-sm text-sm text-muted-foreground">
        {t('errors:appError.body')}
      </p>
      <Button onClick={handleReload}>{t('errors:appError.reload')}</Button>
    </div>
  )
}
