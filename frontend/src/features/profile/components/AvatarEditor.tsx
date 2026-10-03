"use client";

import { useRef, useState, type ChangeEvent } from "react";
import { SquarePen } from "lucide-react";
import type { Me } from "@/features/auth/types";
import { profileApi } from "@/features/profile/api";
import { UserAvatar } from "@/shared/avatar/UserAvatar";
import { t } from "@/shared/i18n";
import { toast } from "@/shared/lib/toast";

// Uploads are cropped to a centred square and scaled to this edge, which
// keeps them far below the backend's 256 KB cap.
const AVATAR_PX = 256;

async function toSquareImage(file: File): Promise<Blob> {
  const bitmap = await createImageBitmap(file);
  const side = Math.min(bitmap.width, bitmap.height);
  const canvas = document.createElement("canvas");
  canvas.width = AVATAR_PX;
  canvas.height = AVATAR_PX;
  const ctx = canvas.getContext("2d");
  if (!ctx) throw new Error("no 2d context");
  ctx.drawImage(
    bitmap,
    (bitmap.width - side) / 2,
    (bitmap.height - side) / 2,
    side,
    side,
    0,
    0,
    AVATAR_PX,
    AVATAR_PX,
  );
  bitmap.close();
  const encode = (type: string) =>
    new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, type, 0.85));
  // Browsers that can't encode WebP hand back a PNG (or null) instead.
  const webp = await encode("image/webp");
  if (webp?.type === "image/webp") return webp;
  const jpeg = await encode("image/jpeg");
  if (!jpeg) throw new Error("could not encode image");
  return jpeg;
}

interface AvatarEditorProps {
  me: Me;
  onChange: (me: Me) => void;
}

// The avatar at the top of the edit page: click to pick a new picture,
// or remove the current one to go back to the generated default.
export function AvatarEditor({ me, onChange }: AvatarEditorProps) {
  const input = useRef<HTMLInputElement>(null);
  const [busy, setBusy] = useState(false);

  async function onPick(e: ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    // Reset so picking the same file again still fires a change event.
    e.target.value = "";
    if (!file) return;
    setBusy(true);
    try {
      let image: Blob;
      try {
        image = await toSquareImage(file);
      } catch {
        toast.error(t("prof_edit_avatar_bad_file"));
        return;
      }
      onChange(await profileApi.uploadAvatar(image));
      toast.success(t("prof_edit_avatar_saved"));
    } catch {
      toast.error(t("prof_edit_avatar_failed"));
    } finally {
      setBusy(false);
    }
  }

  async function remove() {
    setBusy(true);
    try {
      onChange(await profileApi.deleteAvatar());
      toast.success(t("prof_edit_avatar_removed"));
    } catch {
      toast.error(t("prof_edit_avatar_failed"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex flex-col items-center gap-3">
      <button
        type="button"
        onClick={() => input.current?.click()}
        disabled={busy}
        title={t("prof_edit_avatar")}
        aria-label={t("prof_edit_avatar")}
        className="group relative overflow-hidden rounded-2xl focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent disabled:opacity-60"
      >
        <UserAvatar url={me.avatar_url} seed={me.handle} className="size-32 rounded-2xl" />
        <span className="absolute inset-0 grid place-items-center bg-black/0 text-white opacity-0 transition group-hover:bg-black/40 group-hover:opacity-100 group-focus-visible:bg-black/40 group-focus-visible:opacity-100">
          <SquarePen size={22} />
        </span>
      </button>
      <input
        ref={input}
        type="file"
        accept="image/png,image/jpeg,image/webp"
        onChange={onPick}
        className="hidden"
        aria-hidden
        tabIndex={-1}
      />
      {me.avatar_url && (
        <button
          type="button"
          onClick={remove}
          disabled={busy}
          className="text-xs text-muted hover:text-danger disabled:opacity-60"
        >
          {t("prof_edit_avatar_remove")}
        </button>
      )}
    </div>
  );
}
