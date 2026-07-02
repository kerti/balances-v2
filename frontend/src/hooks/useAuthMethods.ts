import { useQuery } from "@tanstack/react-query";
import { api } from "@/api/client";

// AuthMethods is the public pre-auth config surface (ADR-0039): which identity
// providers this instance has enabled. The sign-in screen renders only the
// buttons/forms for live providers — Google-only (hosted), local-only (the
// minimal self-host posture), or both.
export type AuthMethods = {
  google: boolean;
  local: boolean;
  // Whether emailed self-service password reset is available (#282): true only
  // when local auth and outbound email are both on. The sign-in form hides its
  // "Forgot password?" link when false.
  password_reset: boolean;
  // The public demo posture (ADR-0041, #217). When true, demo_email/demo_password
  // carry the shared demo login — the sign-in form pre-fills them so a visitor can
  // just click Sign in. Absent (not merely empty) when demo_mode is false.
  demo_mode: boolean;
  demo_email?: string;
  demo_password?: string;
};

// useAuthMethods fetches the enabled providers. It is safe to call pre-auth and
// is effectively static for the life of a deployment, so it never refetches on
// focus and stays fresh for the session.
export function useAuthMethods() {
  return useQuery<AuthMethods>({
    queryKey: ["auth-methods"],
    queryFn: () => api<AuthMethods>("/api/auth/methods"),
    staleTime: Infinity,
    refetchOnWindowFocus: false,
  });
}
