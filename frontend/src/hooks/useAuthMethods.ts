import { useQuery } from "@tanstack/react-query";
import { api } from "@/api/client";

// AuthMethods is the public pre-auth config surface (ADR-0039): which identity
// providers this instance has enabled. The sign-in screen renders only the
// buttons/forms for live providers — Google-only (hosted), local-only (the
// minimal self-host posture), or both.
export type AuthMethods = {
  google: boolean;
  local: boolean;
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
