import { useState } from "react";
import { initials } from "@/lib/names";
import { cn } from "@/lib/utils";

type Props = {
  name: string;
  pictureUrl: string | null;
  className?: string;
};

// UserAvatar renders the Google profile picture as a rounded square, falling
// back to the person's initials when there's no picture or the image fails to
// load. referrerPolicy="no-referrer" is required: Google's lh3.googleusercontent
// URLs return 403 when a referrer header is sent. The failed-src state (not a
// boolean) lets a changed pictureUrl re-attempt without an effect to reset it.
export function UserAvatar({ name, pictureUrl, className }: Props) {
  const [failedSrc, setFailedSrc] = useState<string | null>(null);
  const showImage = pictureUrl !== null && failedSrc !== pictureUrl;

  return (
    <div
      data-testid="user-avatar"
      className={cn(
        "flex h-8 w-8 shrink-0 select-none items-center justify-center overflow-hidden rounded-md bg-muted text-xs font-medium text-muted-foreground",
        className,
      )}
    >
      {showImage ? (
        <img
          src={pictureUrl}
          alt={name}
          referrerPolicy="no-referrer"
          className="h-full w-full object-cover"
          onError={() => setFailedSrc(pictureUrl)}
          data-testid="user-avatar-img"
        />
      ) : (
        <span data-testid="user-avatar-fallback">{initials(name)}</span>
      )}
    </div>
  );
}
