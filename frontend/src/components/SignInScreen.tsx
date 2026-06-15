import type { ChangeEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { AppLogo } from '@/components/AppLogo'
import { AppInfo } from '@/components/AppInfo'
import { useLocale } from '@/i18n/useLocale'
import { SUPPORTED_LOCALES, type Locale } from '@/i18n'
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
} from '@/components/ui/card'

// In-language display names, shown regardless of the active UI language so a
// visitor reading the wrong language can still find their option. Mirrors the
// Settings LanguageCard map; kept local so the two screens stay independent.
const LANGUAGE_LABELS: Record<Locale, string> = {
  'en-GB': 'English',
  'id-ID': 'Bahasa Indonesia',
}

export function SignInScreen() {
  const { t } = useTranslation('common')
  // Pre-auth picker is display-only (ADR-0035): switching the language updates
  // i18next + localStorage for the unauthenticated UI and is carried to the
  // backend via the start link's ?lng= so a brand-new account is seeded in the
  // chosen language. It never PATCHes an account — a returning user's saved
  // locale always wins after sign-in. The initial value is navigator-derived by
  // i18next's language detector, so the picker arrives pre-filled.
  const { locale, setLocale } = useLocale()

  const onChange = (e: ChangeEvent<HTMLSelectElement>) => {
    void setLocale(e.target.value as Locale)
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-muted p-6">
      <Card className="w-full max-w-sm">
        <CardHeader>
          <AppLogo className="w-full h-auto" />
          <CardDescription>{t('signIn.tagline')}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-1">
            <Label htmlFor="signin-language">{t('signIn.languageLabel')}</Label>
            <select
              id="signin-language"
              data-testid="signin-language-select"
              className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus-visible:ring-1 focus-visible:ring-ring focus-visible:outline-none"
              value={locale}
              onChange={onChange}
            >
              {SUPPORTED_LOCALES.map((l) => (
                <option key={l} value={l}>
                  {LANGUAGE_LABELS[l]}
                </option>
              ))}
            </select>
          </div>
          <Button asChild className="w-full">
            <a
              href={`/api/auth/google/start?lng=${encodeURIComponent(locale)}`}
              data-testid="signin-google"
            >
              {t('signIn.withGoogle')}
            </a>
          </Button>
        </CardContent>
        {/* Same identity block as the sidebar footer (issue #123) so an
            unauthenticated visitor can still see the version, deploy target,
            and project/maintainer links. */}
        <CardFooter className="border-t pt-4">
          <AppInfo variant="split" />
        </CardFooter>
      </Card>
    </div>
  )
}
