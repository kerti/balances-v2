import { useState } from "react";
import { toast } from "sonner";
import i18n from "@/i18n";

// Copy a one-time link to the clipboard without ever rendering it. The caller
// shows a Copy button and flips its label while `copied` is true; a denied or
// unavailable clipboard raises one error toast and leaves the button usable, so
// the person can allow clipboard access and press it again. The link itself is
// never put on screen (#847): it is a bearer secret, and a visible URL invites
// screenshots and shoulder-surfing for no benefit over the button.
export function useCopyLink() {
  const [copied, setCopied] = useState(false);

  async function copy(url: string) {
    try {
      await navigator.clipboard.writeText(url);
      setCopied(true);
    } catch {
      setCopied(false);
      toast.error(i18n.t("settings:copyLinkFailed"));
    }
  }

  return { copied, copy, reset: () => setCopied(false) };
}
