import { useEffect, useRef } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import type { Me } from "@/hooks/useSession";

// useInviteIgnoredNotice surfaces the gentle, non-blocking explanation when an
// already-onboarded user arrives via a fresh invite link (ADR-0038, #269). The
// OAuth callback can't render UI, so it carries a `?notice=invite_ignored`
// signal on the post-login redirect; here we turn it into a toast that names
// the user's current Household and states the one-Household rule, then strip the
// query param so a refresh doesn't replay it. Fires at most once per mount.
export function useInviteIgnoredNotice(user: Me | null | undefined) {
  const { t } = useTranslation("onboarding");
  const shown = useRef(false);

  useEffect(() => {
    if (!user || shown.current) return;
    const params = new URLSearchParams(window.location.search);
    if (params.get("notice") !== "invite_ignored") return;

    shown.current = true;
    toast(t("alreadyMember", { household: user.household_display_name }));

    // Drop the consumed signal from the URL without a navigation.
    params.delete("notice");
    const qs = params.toString();
    window.history.replaceState(
      window.history.state,
      "",
      window.location.pathname + (qs ? `?${qs}` : "") + window.location.hash,
    );
  }, [user, t]);
}
