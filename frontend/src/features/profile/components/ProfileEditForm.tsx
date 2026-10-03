"use client";

import { useState, type ComponentType, type FormEvent } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  ArrowLeft,
  ArrowRight,
  AtSign,
  Cake,
  FileText,
  Globe,
  Mail,
  MapPin,
  UserRound,
  VenusAndMars,
} from "lucide-react";
import type { Me } from "@/features/auth/types";
import { profileApi, type ProfileUpdate } from "@/features/profile/api";
import { formatBirthday, isGeneratedHandle, linkLabel } from "@/features/profile/lib/fields";
import { ApiError } from "@/shared/api/errors";
import { t, type LocaleKey } from "@/shared/i18n";
import { useLocale } from "@/shared/i18n/client";
import { cn } from "@/shared/lib/cn";
import { toast } from "@/shared/lib/toast";
import { AvatarEditor } from "./AvatarEditor";
import { GithubIcon, LinkedinIcon, XIcon } from "./BrandIcons";

// Limits mirror the backend (internal/account/account.go).
const MAX_NAME = 50;
const MAX_BIO = 280;
const MAX_REGION = 40;
const MAX_URL = 200;

const GENDER_LABEL_KEY: Record<Exclude<Me["gender"], "">, LocaleKey> = {
  male: "prof_edit_gender_male",
  female: "prof_edit_gender_female",
  non_binary: "prof_edit_gender_non_binary",
  other: "prof_edit_gender_other",
};

type FieldKey = keyof ProfileUpdate;
type Icon = ComponentType<{ size?: number; className?: string }>;

interface Field {
  // Omitted for rows that only display a value (username, email).
  key?: FieldKey;
  id: string;
  labelKey: LocaleKey;
  icon: Icon;
  input?: "text" | "textarea" | "select" | "date";
  placeholderKey?: LocaleKey;
  maxLength?: number;
  required?: boolean;
  // What the collapsed row shows; defaults to the raw value.
  display?: (value: string) => string;
}

interface Section {
  titleKey: LocaleKey;
  descKey: LocaleKey;
  fields: Field[];
}

const link = (id: string, key: FieldKey, labelKey: LocaleKey, icon: Icon, placeholderKey: LocaleKey): Field => ({
  key,
  id,
  labelKey,
  icon,
  input: "text",
  placeholderKey,
  maxLength: MAX_URL,
  display: linkLabel,
});

const SECTIONS: Section[] = [
  {
    titleKey: "prof_edit_general_title",
    descKey: "prof_edit_general_desc",
    fields: [
      { key: "display_name", id: "name", labelKey: "prof_edit_name", icon: UserRound, input: "text", maxLength: MAX_NAME, required: true },
      { id: "username", labelKey: "prof_edit_username", icon: AtSign },
      { key: "bio", id: "bio", labelKey: "prof_edit_bio", icon: FileText, input: "textarea", placeholderKey: "prof_edit_bio_placeholder", maxLength: MAX_BIO },
      { key: "region", id: "region", labelKey: "prof_edit_region", icon: MapPin, input: "text", placeholderKey: "prof_edit_region_placeholder", maxLength: MAX_REGION },
    ],
  },
  {
    titleKey: "prof_edit_links_title",
    descKey: "prof_edit_links_desc",
    fields: [
      link("website", "website_url", "prof_edit_website", Globe, "prof_edit_website_placeholder"),
      link("github", "github_url", "prof_edit_github", GithubIcon, "prof_edit_github_placeholder"),
      link("linkedin", "linkedin_url", "prof_edit_linkedin", LinkedinIcon, "prof_edit_linkedin_placeholder"),
      link("x", "x_url", "prof_edit_x", XIcon, "prof_edit_x_placeholder"),
    ],
  },
  {
    titleKey: "prof_edit_private_title",
    descKey: "prof_edit_private_desc",
    fields: [
      { id: "email", labelKey: "prof_edit_email", icon: Mail },
      {
        key: "gender",
        id: "gender",
        labelKey: "prof_edit_gender",
        icon: VenusAndMars,
        input: "select",
        display: (v) => (v ? t(GENDER_LABEL_KEY[v as keyof typeof GENDER_LABEL_KEY]) : ""),
      },
      { key: "birthday", id: "birthday", labelKey: "prof_edit_birthday", icon: Cake, input: "date", display: formatBirthday },
    ],
  },
];

function readOnlyValue(me: Me, id: string): string {
  if (id === "email") return me.email;
  // A minted handle is an id, not a name: never show it.
  return isGeneratedHandle(me.handle) ? "" : `@${me.handle}`;
}

// The profile edit page body: avatar on top, then sections of rows. A row
// shows its current value; clicking it swaps the row for an inline editor
// that saves just that field.
export function ProfileEditForm({ initial }: { initial: Me }) {
  useLocale();
  const router = useRouter();
  const [me, setMe] = useState(initial);
  const [openId, setOpenId] = useState<string | null>(null);

  // The nav (avatar, display name) is server-rendered from /api/me.
  function onSaved(next: Me) {
    setMe(next);
    router.refresh();
  }

  return (
    <div className="mx-auto max-w-[820px] space-y-8 px-4 py-8 sm:px-6 sm:py-10">
      <Link href="/profile" className="inline-flex items-center gap-1.5 text-sm text-muted hover:text-ink">
        <ArrowLeft size={15} />
        {t("prof_edit_back")}
      </Link>

      <AvatarEditor me={me} onChange={onSaved} />

      {SECTIONS.map((section) => (
        <section key={section.titleKey}>
          <h2 className="text-base font-semibold">{t(section.titleKey)}</h2>
          <p className="mt-0.5 text-sm text-muted">{t(section.descKey)}</p>
          <ul className="mt-3 divide-y divide-divider overflow-hidden rounded-xl border border-divider bg-surface">
            {section.fields.map((field) => {
              const key = field.key;
              // Nothing to show for a minted handle; drop the row entirely.
              if (!key && !readOnlyValue(me, field.id)) return null;
              return (
                <li key={field.id}>
                  {!key ? (
                    <RowShell field={field} value={readOnlyValue(me, field.id)} />
                  ) : openId === field.id ? (
                    <RowEditor
                      field={field}
                      fieldKey={key}
                      value={me[key]}
                      onCancel={() => setOpenId(null)}
                      onSaved={(next) => {
                        onSaved(next);
                        setOpenId(null);
                      }}
                    />
                  ) : (
                    <RowShell
                      field={field}
                      value={field.display ? field.display(me[key]) : me[key]}
                      onOpen={() => setOpenId(field.id)}
                    />
                  )}
                </li>
              );
            })}
          </ul>
        </section>
      ))}
    </div>
  );
}

const ROW = "flex w-full items-center gap-4 px-5 py-4 text-left";

// A collapsed row: icon, label, current value, and an arrow when editable.
function RowShell({ field, value, onOpen }: { field: Field; value: string; onOpen?: () => void }) {
  const body = (
    <>
      <field.icon size={18} className="shrink-0 text-muted" />
      <span className="shrink-0 font-medium">{t(field.labelKey)}</span>
      <span className="min-w-0 flex-1 truncate text-sm text-muted">{value}</span>
      {onOpen && <ArrowRight size={18} className="shrink-0 text-muted" />}
    </>
  );
  if (!onOpen) return <div className={ROW}>{body}</div>;
  return (
    <button type="button" onClick={onOpen} className={cn(ROW, "transition-colors hover:bg-surface-2")}>
      {body}
    </button>
  );
}

const CONTROL =
  "block w-full rounded-md border border-divider bg-canvas px-3 text-sm text-ink placeholder:text-faint focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/50";

interface RowEditorProps {
  field: Field;
  fieldKey: FieldKey;
  value: string;
  onCancel: () => void;
  onSaved: (me: Me) => void;
}

function RowEditor({ field, fieldKey, value, onCancel, onSaved }: RowEditorProps) {
  const [draft, setDraft] = useState(value);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const inputId = `pf-${field.id}`;
  const placeholder = field.placeholderKey ? t(field.placeholderKey) : undefined;

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (field.required && !draft.trim()) {
      setError(t("prof_edit_name_required"));
      return;
    }
    setSaving(true);
    setError(null);
    try {
      const next = await profileApi.updateMe({ [fieldKey]: draft });
      toast.success(t("prof_edit_saved"));
      onSaved(next);
    } catch (err) {
      // The backend's 400 messages say what is wrong with the value.
      setError(err instanceof ApiError && err.status === 400 ? err.message : t("prof_edit_failed"));
      setSaving(false);
    }
  }

  return (
    <form onSubmit={submit} className="space-y-3 px-5 py-4">
      <label htmlFor={inputId} className="flex items-center gap-4 font-medium">
        <field.icon size={18} className="shrink-0 text-muted" />
        {t(field.labelKey)}
      </label>
      <div className="sm:pl-[34px]">
        {field.input === "textarea" ? (
          <>
            <textarea
              id={inputId}
              autoFocus
              value={draft}
              maxLength={field.maxLength}
              rows={4}
              placeholder={placeholder}
              onChange={(e) => setDraft(e.target.value)}
              className={cn(CONTROL, "resize-none py-2")}
            />
            <p className="mt-1 text-right text-xs tabular-nums text-faint">
              {draft.length}/{field.maxLength}
            </p>
          </>
        ) : field.input === "select" ? (
          <select
            id={inputId}
            autoFocus
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            className={cn(CONTROL, "h-10")}
          >
            <option value="">{t("prof_edit_gender_unset")}</option>
            {Object.entries(GENDER_LABEL_KEY).map(([v, labelKey]) => (
              <option key={v} value={v}>
                {t(labelKey)}
              </option>
            ))}
          </select>
        ) : (
          <input
            id={inputId}
            autoFocus
            type={field.input}
            value={draft}
            maxLength={field.maxLength}
            placeholder={placeholder}
            // Birthdays: the native picker should not offer future dates.
            max={field.input === "date" ? new Date().toISOString().slice(0, 10) : undefined}
            min={field.input === "date" ? "1900-01-01" : undefined}
            onChange={(e) => setDraft(e.target.value)}
            className={cn(CONTROL, "h-10")}
          />
        )}
        {error && (
          <p role="alert" className="mt-2 text-sm text-danger">
            {error}
          </p>
        )}
        <div className="mt-3 flex justify-end gap-2">
          <button type="button" onClick={onCancel} className="btn-pill btn-pill--ghost text-sm">
            {t("prof_edit_cancel")}
          </button>
          <button
            type="submit"
            disabled={saving || draft === value}
            className="btn-pill btn-pill--primary text-sm disabled:opacity-60"
          >
            {saving ? t("prof_edit_saving") : t("prof_edit_save")}
          </button>
        </div>
      </div>
    </form>
  );
}
