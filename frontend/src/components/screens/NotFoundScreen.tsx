import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { Button } from "@/components/ui/button";
import { routes } from "@/lib/routes";

// NotFoundScreen is the catch-all route's page, rendered inside the app shell.
// An unknown URL used to redirect silently to the dashboard, so a mistyped
// address gave no hint the page didn't exist.
export function NotFoundScreen() {
  const { t } = useTranslation("common");
  return (
    <div className="space-y-4" data-testid="not-found-screen">
      <h1 className="text-2xl font-semibold tracking-tight">{t("notFound.title")}</h1>
      <p className="text-muted-foreground">{t("notFound.body")}</p>
      <Button asChild>
        <Link to={routes.dashboard} data-testid="not-found-dashboard-link">
          {t("notFound.backToDashboard")}
        </Link>
      </Button>
    </div>
  );
}
