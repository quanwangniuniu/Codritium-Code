"use client";

import { useCallback, useEffect, useRef, useState, type KeyboardEvent } from "react";
import { useRouter } from "next/navigation";
import { X } from "lucide-react";
import { t } from "@/shared/i18n";
import { useLocale } from "@/shared/i18n/client";
import { toast } from "@/shared/lib/toast";
import { cn } from "@/shared/lib/cn";
import {
  FORUM_ANON_SECTIONS,
  FORUM_LIMITS,
  FORUM_SECTIONS,
  FORUM_SECTION_LABEL_KEY,
  forumApi,
  forumErrorMessage,
  type ForumPostInput,
  type ForumSection,
} from "@/features/forum/api";
import { ForumMarkdown } from "@/features/forum/components/ForumMarkdown";

interface ForumPostEditorProps {
  // Present when editing an existing post.
  postId?: string;
  initial?: ForumPostInput;
  defaultSection?: ForumSection;
  problems: { slug: string; title: string }[];
}

// The editor's fields as saved to localStorage between visits.
interface Draft {
  section: ForumSection;
  title: string;
  body: string;
  tags: string[];
  anonymous: boolean;
  problemSlug: string;
}

function draftKey(postId?: string): string {
  return postId ? `forum-draft:edit:${postId}` : "forum-draft:new";
}

// Storage can be unavailable (private mode, blocked site data); drafts are a
// convenience, so failures are ignored.
function loadDraft(key: string): Draft | null {
  try {
    const raw = window.localStorage.getItem(key);
    if (!raw) return null;
    const d = JSON.parse(raw) as Partial<Draft>;
    if (typeof d.title !== "string" || typeof d.body !== "string") return null;
    return {
      section: FORUM_SECTIONS.includes(d.section as ForumSection) ? (d.section as ForumSection) : "general",
      title: d.title,
      body: d.body,
      tags: Array.isArray(d.tags) ? d.tags.filter((x): x is string => typeof x === "string") : [],
      anonymous: d.anonymous === true,
      problemSlug: typeof d.problemSlug === "string" ? d.problemSlug : "",
    };
  } catch {
    return null;
  }
}

function saveDraft(key: string, d: Draft | null) {
  try {
    if (d) window.localStorage.setItem(key, JSON.stringify(d));
    else window.localStorage.removeItem(key);
  } catch {
    // ignore
  }
}

function sameDraft(a: Draft, b: Draft): boolean {
  return JSON.stringify(a) === JSON.stringify(b);
}

function normalizeTag(raw: string): string {
  return raw.trim().replace(/^#/, "").toLowerCase().split(/\s+/).join("-");
}

export function ForumPostEditor({ postId, initial, defaultSection, problems }: ForumPostEditorProps) {
  useLocale();
  const router = useRouter();
  // What the form starts from (and returns to when a draft is discarded).
  const [start] = useState<Draft>(() => ({
    section: initial?.section ?? defaultSection ?? "general",
    title: initial?.title ?? "",
    body: initial?.body_md ?? "",
    tags: initial?.tags ?? [],
    anonymous: initial?.is_anonymous ?? false,
    problemSlug: initial?.problem_slug ?? "",
  }));
  const [section, setSection] = useState<ForumSection>(start.section);
  const [title, setTitle] = useState(start.title);
  const [body, setBody] = useState(start.body);
  const [tags, setTags] = useState<string[]>(start.tags);
  const [tagDraft, setTagDraft] = useState("");
  const [anonymous, setAnonymous] = useState(start.anonymous);
  const [problemSlug, setProblemSlug] = useState(start.problemSlug);
  const [tab, setTab] = useState<"write" | "preview">("write");
  const [busy, setBusy] = useState(false);
  const [restored, setRestored] = useState(false);

  // Autosave: restore an unsaved draft on mount, then keep it updated while
  // typing. Saving waits for the restore so it can't overwrite the draft.
  const key = draftKey(postId);
  const loaded = useRef(false);
  const apply = useCallback((d: Draft) => {
    setSection(d.section);
    setTitle(d.title);
    setBody(d.body);
    setTags(d.tags);
    setAnonymous(d.anonymous);
    setProblemSlug(d.problemSlug);
  }, []);
  useEffect(() => {
    const d = loadDraft(key);
    if (d && !sameDraft(d, start)) {
      apply(d);
      setRestored(true);
    }
    loaded.current = true;
  }, [apply, key, start]);
  useEffect(() => {
    if (!loaded.current) return;
    const current: Draft = { section, title, body, tags, anonymous, problemSlug };
    const timer = setTimeout(() => saveDraft(key, sameDraft(current, start) ? null : current), 500);
    return () => clearTimeout(timer);
  }, [key, start, section, title, body, tags, anonymous, problemSlug]);

  function discardDraft() {
    apply(start);
    saveDraft(key, null);
    setRestored(false);
  }

  const anonAllowed = FORUM_ANON_SECTIONS.includes(section);
  const titleLen = title.trim().length;
  const titleOk = titleLen >= FORUM_LIMITS.titleMin && titleLen <= FORUM_LIMITS.titleMax;
  const canSubmit = titleOk && body.trim().length > 0 && !busy;

  function addTag(raw: string) {
    const tag = normalizeTag(raw);
    if (!tag || tags.includes(tag) || tags.length >= FORUM_LIMITS.maxTags) return;
    if (!/^[a-z0-9][a-z0-9+#.-]{0,23}$/.test(tag)) {
      toast.warn(t("forum_err_invalid_tag"));
      return;
    }
    setTags([...tags, tag]);
  }

  function onTagKey(e: KeyboardEvent<HTMLInputElement>) {
    if (e.key === "Enter" || e.key === ",") {
      e.preventDefault();
      addTag(tagDraft);
      setTagDraft("");
    } else if (e.key === "Backspace" && !tagDraft && tags.length > 0) {
      setTags(tags.slice(0, -1));
    }
  }

  async function submit() {
    if (!canSubmit) return;
    // Commit a half-typed tag instead of silently dropping it.
    const pending = normalizeTag(tagDraft);
    const finalTags = pending && !tags.includes(pending) ? [...tags, pending].slice(0, FORUM_LIMITS.maxTags) : tags;
    const input: ForumPostInput = {
      section,
      title: title.trim(),
      body_md: body,
      tags: finalTags,
      is_anonymous: anonAllowed && anonymous,
      problem_slug: problemSlug || null,
    };
    setBusy(true);
    try {
      const post = postId ? await forumApi.updatePost(postId, input) : await forumApi.createPost(input);
      // Stop autosave first so the pending save can't bring the draft back.
      loaded.current = false;
      saveDraft(key, null);
      toast.success(postId ? t("forum_post_updated") : t("forum_post_published"));
      router.push(`/forums/${post.id}`);
      router.refresh();
    } catch (e) {
      toast.error(forumErrorMessage(e));
      setBusy(false);
    }
  }

  const field =
    "w-full rounded-md border border-divider bg-surface px-3 py-2 text-sm text-ink outline-none placeholder:text-faint focus:border-accent";

  return (
    <form
      className="space-y-5"
      onSubmit={(e) => {
        e.preventDefault();
        void submit();
      }}
    >
      {restored && (
        <div className="flex flex-wrap items-center gap-3 rounded-md border border-divider bg-surface-2 px-3 py-2 text-sm text-muted">
          {t("forum_draft_restored")}
          <button type="button" onClick={discardDraft} className="ml-auto text-accent hover:underline">
            {t("forum_draft_discard")}
          </button>
        </div>
      )}
      <div className="space-y-1.5">
        <input
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          placeholder={t("forum_title_placeholder")}
          aria-label={t("forum_title_label")}
          maxLength={FORUM_LIMITS.titleMax}
          className="w-full border-0 border-b border-divider bg-transparent px-0 pb-2 text-2xl font-semibold text-ink outline-none placeholder:text-faint focus:border-accent"
        />
        <p className={cn("text-xs", titleLen > 0 && !titleOk ? "text-danger" : "text-faint")}>
          {t("forum_title_hint_fmt", {
            params: { min: FORUM_LIMITS.titleMin, max: FORUM_LIMITS.titleMax, n: titleLen },
          })}
        </p>
      </div>

      <div className="grid gap-4 sm:grid-cols-2">
        <label className="space-y-1.5 text-sm">
          <span className="text-muted">{t("forum_section_label")}</span>
          <select
            value={section}
            onChange={(e) => setSection(e.target.value as ForumSection)}
            className={field}
          >
            {FORUM_SECTIONS.map((s) => (
              <option key={s} value={s}>
                {t(FORUM_SECTION_LABEL_KEY[s])}
              </option>
            ))}
          </select>
        </label>
        <label className="space-y-1.5 text-sm">
          <span className="text-muted">{t("forum_problem_label")}</span>
          <select value={problemSlug} onChange={(e) => setProblemSlug(e.target.value)} className={field}>
            <option value="">{t("forum_problem_none")}</option>
            {problems.map((p) => (
              <option key={p.slug} value={p.slug}>
                {p.title}
              </option>
            ))}
          </select>
        </label>
      </div>

      <div className="space-y-1.5 text-sm">
        <span className="text-muted">
          {t("forum_tags_label_fmt", { params: { max: FORUM_LIMITS.maxTags } })}
        </span>
        <div className={cn(field, "flex flex-wrap items-center gap-1.5 py-1.5")}>
          {tags.map((tag) => (
            <span key={tag} className="inline-flex items-center gap-1 rounded-full bg-surface-2 px-2 py-0.5 text-xs text-ink">
              #{tag}
              <button
                type="button"
                aria-label={t("forum_remove_tag_fmt", { params: { tag } })}
                onClick={() => setTags(tags.filter((x) => x !== tag))}
                className="text-faint hover:text-ink"
              >
                <X size={12} />
              </button>
            </span>
          ))}
          {tags.length < FORUM_LIMITS.maxTags && (
            <input
              value={tagDraft}
              onChange={(e) => setTagDraft(e.target.value)}
              onKeyDown={onTagKey}
              onBlur={() => {
                addTag(tagDraft);
                setTagDraft("");
              }}
              placeholder={tags.length === 0 ? t("forum_tags_placeholder") : ""}
              aria-label={t("forum_tags_placeholder")}
              className="min-w-[8rem] flex-1 bg-transparent py-0.5 text-sm outline-none placeholder:text-faint"
            />
          )}
        </div>
      </div>

      <div className="overflow-hidden rounded-md border border-divider">
        <div className="flex items-center gap-1 border-b border-divider bg-surface-2/60 px-2 py-1.5" role="tablist">
          {(["write", "preview"] as const).map((v) => (
            <button
              key={v}
              type="button"
              role="tab"
              aria-selected={tab === v}
              onClick={() => setTab(v)}
              className={cn(
                "rounded-md px-3 py-1 text-sm",
                tab === v ? "bg-surface font-medium text-ink shadow-sm" : "text-muted hover:text-ink",
              )}
            >
              {v === "write" ? t("forum_write") : t("forum_preview")}
            </button>
          ))}
          <span className="ml-auto text-xs text-faint">{t("forum_markdown_hint")}</span>
        </div>
        {tab === "write" ? (
          <textarea
            value={body}
            onChange={(e) => setBody(e.target.value)}
            placeholder={t("forum_body_placeholder")}
            aria-label={t("forum_body_label")}
            maxLength={FORUM_LIMITS.bodyMax}
            rows={16}
            className="block w-full resize-y bg-surface p-3 font-mono text-sm text-ink outline-none placeholder:text-faint"
          />
        ) : (
          <div className="min-h-[24rem] bg-surface p-4">
            {body.trim() ? (
              <ForumMarkdown source={body} />
            ) : (
              <p className="text-sm text-faint">{t("forum_preview_empty")}</p>
            )}
          </div>
        )}
      </div>

      <div className="flex flex-wrap items-center gap-4">
        {anonAllowed && (
          <label className="inline-flex items-center gap-2 text-sm text-muted">
            <input type="checkbox" checked={anonymous} onChange={(e) => setAnonymous(e.target.checked)} />
            {t("forum_post_anonymously")}
          </label>
        )}
        <div className="ml-auto flex gap-2">
          <button
            type="button"
            onClick={() => router.back()}
            className="rounded-md px-4 py-2 text-sm text-muted hover:text-ink"
          >
            {t("cancel")}
          </button>
          <button
            type="submit"
            disabled={!canSubmit}
            className="rounded-md bg-success px-5 py-2 text-sm font-medium text-white hover:opacity-90 disabled:opacity-50"
          >
            {busy ? t("loading") : postId ? t("forum_save") : t("forum_publish")}
          </button>
        </div>
      </div>
    </form>
  );
}
