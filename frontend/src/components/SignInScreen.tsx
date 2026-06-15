import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { AppLogo } from '@/components/AppLogo'
import { AppInfo } from '@/components/AppInfo'
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
} from '@/components/ui/card'

export function SignInScreen() {
  const { t } = useTranslation('common')
  return (
    <div className="min-h-screen flex items-center justify-center bg-muted p-6">
      <Card className="w-full max-w-sm">
        <CardHeader>
          <AppLogo className="w-full h-auto" />
          <CardDescription>{t('signIn.tagline')}</CardDescription>
        </CardHeader>
        <CardContent>
          <Button asChild className="w-full">
            <a href="/api/auth/google/start" data-testid="signin-google">
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
