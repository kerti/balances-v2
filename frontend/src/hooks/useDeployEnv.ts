import { useQuery } from "@tanstack/react-query";
import { api } from "@/api/client";
import { resolveDeployEnv, type DeployEnv } from "@/lib/appInfo";

// /healthz is public (no session cookie needed), which matters here: AppInfo
// renders on pre-auth screens (sign-in, onboarding, invite-accept, reset) as
// well as the signed-in sidebar footer.
type Healthz = { deploy_env: string };

// useDeployEnv resolves the runtime deploy target (#354): the same built
// image is deployed to every environment, so the label can't be baked in at
// build time like APP_VERSION is. Returns "local" until the first fetch
// resolves.
export function useDeployEnv(): DeployEnv {
  const { data } = useQuery({
    queryKey: ["healthz"],
    queryFn: () => api<Healthz>("/healthz"),
    staleTime: 60_000,
  });

  return resolveDeployEnv(data?.deploy_env);
}
