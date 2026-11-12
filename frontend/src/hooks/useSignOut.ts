import { useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router";
import { api } from "@/api/client";
import { routes } from "@/lib/routes";

// The one sign-out: the header button (≥1024px) and the drawer's account
// section (<1024px) both call this.
export function useSignOut() {
  const qc = useQueryClient();
  const navigate = useNavigate();

  return async function signOut() {
    try {
      await api("/api/auth/logout", { method: "POST" });
    } finally {
      // Land back on the dashboard route before the session flips to
      // signed-out — otherwise App swaps RouterProvider for SignInScreen
      // while leaving whatever path the user was on (e.g. /settings) in the
      // address bar.
      navigate(routes.dashboard, { replace: true });
      await qc.invalidateQueries({ queryKey: ["session"] });
    }
  };
}
