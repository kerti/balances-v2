/// <reference types="vite/client" />

// Build-time app identity baked into the SPA bundle (issue #75). Injected by
// the deploy pipeline as VITE_ build args (see Dockerfile + deploy.yml); unset
// on a local `npm run dev`/`npm run build`, hence optional.
interface ImportMetaEnv {
  readonly VITE_APP_VERSION?: string;
  readonly VITE_DEPLOY_ENV?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
