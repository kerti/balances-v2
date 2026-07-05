import { setupServer } from "msw/node";

// Shared MSW server for the component-test tier. Handlers are registered
// per-test with `server.use(...)`; `setup.ts` owns its listen/reset/close
// lifecycle. Any request with no matching handler fails the test loudly
// (`onUnhandledRequest: "error"`) so a missing stub can't silently hang a
// react-query fetch.
export const server = setupServer();
