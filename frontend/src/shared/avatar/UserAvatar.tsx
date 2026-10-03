import { cn } from "@/shared/lib/cn";
import { avatarSrc } from "./defaultAvatar";

interface UserAvatarProps {
  // users.avatar_url as the API returned it; empty shows the generated one.
  url?: string | null;
  // The user's handle. Every surface must pass the same seed for a user so
  // their generated avatar matches across the nav, profile, forum and comments.
  seed: string;
  // Rendered width and height in px. Omit to size with className instead.
  size?: number;
  className?: string;
}

// The one way to draw a user's picture. Decorative: the name is always
// rendered next to it, so alt stays empty.
export function UserAvatar({ url, seed, size, className }: UserAvatarProps) {
  return (
    // eslint-disable-next-line @next/next/no-img-element
    <img
      src={avatarSrc(url, seed)}
      alt=""
      width={size}
      height={size}
      style={size ? { width: size, height: size } : undefined}
      className={cn("shrink-0 rounded-full object-cover", className)}
    />
  );
}
