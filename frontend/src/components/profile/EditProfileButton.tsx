"use client";

import { useRef, useState, type FormEvent } from "react";
import { useRouter } from "next/navigation";
import { X } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Backend } from "@/lib/api";
import { t } from "@/shared/i18n";
import { toast } from "@/lib/toast";

// Limits mirror the backend (handlers/profile.go).
const MAX_NAME = 50;
const MAX_BIO = 280;
const MAX_REGION = 40;

interface EditProfileButtonProps {
  displayName: string;
  bio: string;
  region: string;
}

export function EditProfileButton({ displayName, bio, region }: EditProfileButtonProps) {
  const router = useRouter();
  const dialogRef = useRef<HTMLDialogElement>(null);
  const [form, setForm] = useState({ display_name: displayName, bio, region });
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const open = () => {
    setForm({ display_name: displayName, bio, region });
    setError(null);
    dialogRef.current?.showModal();
  };
  const close = () => dialogRef.current?.close();

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    if (!form.display_name.trim()) {
      setError(t("prof_edit_name_required"));
      return;
    }
    setSaving(true);
    setError(null);
    try {
      await Backend.updateMe(form);
      close();
      toast.success(t("prof_edit_saved"));
      router.refresh();
    } catch {
      setError(t("prof_edit_failed"));
    } finally {
      setSaving(false);
    }
  };

  return (
    <>
      <button
        type="button"
        onClick={open}
        className="w-full rounded-xl bg-home-green-soft text-home-green font-medium py-2.5 text-sm hover:brightness-95 transition"
      >
        {t("prof_edit_btn")}
      </button>

      <dialog
        ref={dialogRef}
        aria-labelledby="edit-profile-title"
        className="m-auto w-[min(460px,calc(100vw-32px))] rounded-2xl border border-divider bg-surface text-ink p-0 shadow-2xl backdrop:bg-black/50"
      >
        <form onSubmit={submit} className="p-6 space-y-4">
          <div className="flex items-center justify-between">
            <h2 id="edit-profile-title" className="text-lg font-semibold">
              {t("prof_edit_title")}
            </h2>
            <button type="button" onClick={close} aria-label={t("cancel")} className="nav-icon-btn">
              <X size={16} />
            </button>
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="pf-name">{t("prof_edit_name")}</Label>
            <Input
              id="pf-name"
              value={form.display_name}
              maxLength={MAX_NAME}
              required
              onChange={(e) => setForm({ ...form, display_name: e.target.value })}
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="pf-bio">{t("prof_edit_bio")}</Label>
            <textarea
              id="pf-bio"
              value={form.bio}
              maxLength={MAX_BIO}
              rows={4}
              placeholder={t("prof_edit_bio_placeholder")}
              onChange={(e) => setForm({ ...form, bio: e.target.value })}
              className="block w-full rounded-md border border-divider bg-canvas px-3 py-2 text-sm placeholder:text-faint focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/50 resize-none"
            />
            <p className="text-xs text-faint text-right tabular-nums">
              {form.bio.length}/{MAX_BIO}
            </p>
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="pf-region">{t("prof_edit_region")}</Label>
            <Input
              id="pf-region"
              value={form.region}
              maxLength={MAX_REGION}
              placeholder={t("prof_edit_region_placeholder")}
              onChange={(e) => setForm({ ...form, region: e.target.value })}
            />
          </div>

          {error && <p className="text-sm text-danger">{error}</p>}

          <div className="flex justify-end gap-2 pt-2">
            <button type="button" onClick={close} className="btn-pill btn-pill--ghost text-sm">
              {t("cancel")}
            </button>
            <button type="submit" disabled={saving} className="btn-pill btn-pill--primary text-sm disabled:opacity-60">
              {saving ? t("prof_edit_saving") : t("prof_edit_save")}
            </button>
          </div>
        </form>
      </dialog>
    </>
  );
}
