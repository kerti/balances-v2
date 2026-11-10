// A recording stand-in for sonner so a component test can assert how many toasts
// a write produced and what they carried (ADR-0058 §4: exactly one per write).
// Use it as
//   vi.mock("sonner", async () => (await import("@/test/sonnerMock")).sonnerMock());
// and read the calls back through `toast` imported from "sonner".
import { vi } from "vitest";

export function sonnerMock() {
  const toast = Object.assign(vi.fn(), {
    success: vi.fn(),
    error: vi.fn(),
    warning: vi.fn(),
    loading: vi.fn(() => "toast-id"),
    dismiss: vi.fn(),
  });
  return { toast };
}

type ToastOptions = { duration?: number; action?: { label: string; onClick: () => void } };

// Every toast raised so far, whichever variant produced it.
export function raisedToasts(toast: ReturnType<typeof sonnerMock>["toast"]) {
  return [toast, toast.success, toast.error, toast.warning, toast.loading].flatMap((fn) =>
    fn.mock.calls.map(([message, options]) => ({
      message: message as string,
      options: (options ?? {}) as ToastOptions,
    })),
  );
}
