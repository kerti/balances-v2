import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { api, ApiError } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { AppLogo } from '@/components/AppLogo'
import { AppInfo } from '@/components/AppInfo'
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'

// onboardingInvite mirrors the backend's joinable-Household row. Empty in the
// founder slice (#267); the join UI lands in #268.
type OnboardingInvite = {
  invitation_id: string
  household_id: string
  household_name: string
  inviter_name: string
  hint: boolean
}

type OnboardingOptions = {
  email: string
  display_name: string
  suggested_household_name: string
  invitations: OnboardingInvite[]
}

// OnboardingScreen is the post-auth gate (ADR-0038). It is rendered by App.tsx
// for a visitor holding an onboarding handshake cookie but no session — the
// account does not exist until a choice commits here. This slice (#267)
// implements the founder path; the join rows + found-confirm dialog arrive in
// #268. A missing/expired handshake answers 401 from /options, which we surface
// as a "sign in again" prompt rather than a dead screen.
export function OnboardingScreen() {
  const { t } = useTranslation('onboarding')
  const queryClient = useQueryClient()
  // `null` means "untouched" — the field then shows the server's suggestion as
  // soon as it loads. Once the user types, their value takes over. Derived
  // rather than seeded via an effect to avoid a cascading setState-in-effect;
  // an empty/whitespace value still falls back to the suggestion server-side.
  const [override, setOverride] = useState<string | null>(null)

  const options = useQuery<OnboardingOptions>({
    queryKey: ['onboarding-options'],
    queryFn: () => api<OnboardingOptions>('/api/onboarding/options'),
    retry: false,
  })

  const householdName =
    override ?? options.data?.suggested_household_name ?? ''

  const found = useMutation({
    mutationFn: () =>
      api('/api/onboarding/choice', {
        method: 'POST',
        body: JSON.stringify({ found: true, display_name: householdName }),
      }),
    onSuccess: () => {
      // The commit set the real session cookie; re-running the session query
      // flips App.tsx over to the authed router, landing on the dashboard.
      void queryClient.invalidateQueries({ queryKey: ['session'] })
    },
  })

  const expired =
    options.error instanceof ApiError && options.error.status === 401

  return (
    <div className="min-h-screen flex items-center justify-center bg-muted p-6">
      <Card className="w-full max-w-sm" data-testid="onboarding-card">
        <CardHeader>
          <AppLogo className="w-full h-auto" />
          <CardTitle className="pt-2">{t('title')}</CardTitle>
          <CardDescription>{t('subtitle')}</CardDescription>
        </CardHeader>

        <CardContent className="space-y-4">
          {expired ? (
            <div className="space-y-3" data-testid="onboarding-expired">
              <p className="text-sm text-muted-foreground">{t('expired')}</p>
              <Button asChild variant="outline" className="w-full">
                <a href="/">{t('signInAgain')}</a>
              </Button>
            </div>
          ) : (
            <form
              className="space-y-4"
              onSubmit={(e) => {
                e.preventDefault()
                found.mutate()
              }}
            >
              <div className="space-y-1">
                <p className="text-sm font-medium">{t('founder.title')}</p>
                <p className="text-sm text-muted-foreground">
                  {t('founder.description')}
                </p>
              </div>
              <div className="space-y-1">
                <Label htmlFor="onboarding-household-name">
                  {t('founder.nameLabel')}
                </Label>
                <Input
                  id="onboarding-household-name"
                  data-testid="onboarding-household-name"
                  value={householdName}
                  placeholder={t('founder.namePlaceholder')}
                  onChange={(e) => setOverride(e.target.value)}
                  disabled={options.isPending || found.isPending}
                />
              </div>
              {found.isError && (
                <p className="text-sm text-destructive" role="alert">
                  {t('error')}
                </p>
              )}
              <Button
                type="submit"
                className="w-full"
                data-testid="onboarding-found-submit"
                disabled={options.isPending || found.isPending}
              >
                {found.isPending
                  ? t('founder.submitting')
                  : t('founder.submit')}
              </Button>
            </form>
          )}
        </CardContent>

        <CardFooter className="border-t pt-4">
          <AppInfo variant="split" />
        </CardFooter>
      </Card>
    </div>
  )
}
