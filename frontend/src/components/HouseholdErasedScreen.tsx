import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { AppLogo } from "@/components/AppLogo";
import { routes } from "@/lib/routes";

// HouseholdErasedScreen is where the founder lands right after deleting their
// household (ADR-0040/#300). The erase commit clears the session cookie
// instead of re-issuing one — there's no household left to sign back into —
// so this renders outside the authed router like the other pre-session
// screens. Deliberately not the sign-in screen: a founder who just watched a
// confirm-by-name ceremony land on the ordinary sign-in form might wonder
// whether the deletion actually happened.
export function HouseholdErasedScreen() {
  const { t } = useTranslation("common");

  return (
    <div className="min-h-screen flex items-center justify-center bg-muted p-6">
      <Card className="w-full max-w-sm" data-testid="household-erased-screen">
        <CardHeader>
          <AppLogo className="w-full h-auto" />
          <CardTitle>{t("erased.title")}</CardTitle>
          <CardDescription>{t("erased.body")}</CardDescription>
        </CardHeader>
        <CardContent>
          <Button asChild className="w-full">
            <a href={routes.dashboard}>{t("erased.backToSignIn")}</a>
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}
