import { Toaster as Sonner, type ToasterProps } from "sonner";
import { useTheme } from "@/theme/useTheme";

// Toaster is the single toast viewport for the app (mounted once at the root).
// It is the feedback surface for buttonless interactions — autosaving controls
// like the language/theme selects and the position Tag dropdown that mutate on
// change with no Save button to confirm the write landed (issue #54, ADR-0032).
//
// shadcn's canonical toast is sonner; this thin wrapper wires sonner's palette
// to the app's own theme axis (useTheme, not next-themes) so toasts follow the
// light/dark choice, and maps sonner's CSS hooks onto the index.css custom
// properties so toasts inherit the popover surface used elsewhere.
export function Toaster(props: ToasterProps) {
  const { theme } = useTheme();

  return (
    <Sonner
      theme={theme}
      className="toaster group"
      position="bottom-right"
      // Success toasts wear the brand accent (--primary) so the confirmation
      // pops; errors keep the destructive palette. Tailwind's trailing-`!`
      // important modifier wins over sonner's own [data-sonner-toast] rules.
      toastOptions={{
        classNames: {
          success: "bg-primary! text-primary-foreground! border-primary!",
          error: "bg-destructive! text-white! border-destructive!",
        },
      }}
      style={
        {
          "--normal-bg": "var(--popover)",
          "--normal-text": "var(--popover-foreground)",
          "--normal-border": "var(--border)",
        } as React.CSSProperties
      }
      {...props}
    />
  );
}
