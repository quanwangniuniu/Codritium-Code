"use client";

import { useEffect, useId, useRef, useState, type KeyboardEvent, type TextareaHTMLAttributes } from "react";
import { ImagePlus, Loader2 } from "lucide-react";
import { cn } from "@/shared/lib/cn";
import { t } from "@/shared/i18n";
import { toast } from "@/shared/lib/toast";
import { UserAvatar } from "@/shared/avatar/UserAvatar";
import { forumApi, forumErrorMessage, type MentionSuggestion } from "@/features/forum/api";
import { activeMention, type ActiveMention } from "@/features/forum/mentions";
import { altFromFileName, isImageFile, uploadForumImage } from "@/features/forum/images";

interface MentionTextareaProps
  extends Omit<TextareaHTMLAttributes<HTMLTextAreaElement>, "value" | "onChange"> {
  value: string;
  onValueChange: (value: string) => void;
  // Ranks people already in this thread first.
  postId?: string;
  // Accept images by paste, drag-and-drop, or an attach button; each
  // uploads and lands in the text as markdown.
  allowImages?: boolean;
}

let uploadSeq = 0;

// A textarea with @-mention autocomplete. Typing "@" offers people in the
// thread; typing more searches handles and display names. Arrow keys move,
// Enter or Tab picks, Escape dismisses. With allowImages it also takes
// images.
export function MentionTextarea({
  value,
  onValueChange,
  postId,
  allowImages,
  onKeyDown,
  className,
  ...rest
}: MentionTextareaProps) {
  const ref = useRef<HTMLTextAreaElement>(null);
  const fileInput = useRef<HTMLInputElement>(null);
  // Uploads finish after more typing, so they edit the latest text.
  const latest = useRef(value);
  latest.current = value;
  const [uploading, setUploading] = useState(0);
  const listId = useId();
  const [mention, setMention] = useState<ActiveMention | null>(null);
  const [users, setUsers] = useState<MentionSuggestion[]>([]);
  const [index, setIndex] = useState(0);
  // Caret to restore after inserting a pick.
  const caretAfter = useRef<number | null>(null);

  const query = mention?.query;
  useEffect(() => {
    if (query === undefined || (query === "" && !postId)) {
      setUsers([]);
      return;
    }
    let cancelled = false;
    const timer = setTimeout(() => {
      forumApi
        .suggestMentions(query, postId)
        .then((r) => {
          if (cancelled) return;
          setUsers(r.users);
          setIndex(0);
        })
        .catch(() => !cancelled && setUsers([]));
    }, 150);
    return () => {
      cancelled = true;
      clearTimeout(timer);
    };
  }, [query, postId]);

  // An autofocused box (e.g. a reply prefilled with "@handle ") starts with
  // the caret at the end rather than before the prefill.
  const autoFocus = rest.autoFocus;
  useEffect(() => {
    const el = ref.current;
    if (autoFocus && el) el.setSelectionRange(el.value.length, el.value.length);
  }, [autoFocus]);

  useEffect(() => {
    const pos = caretAfter.current;
    if (pos === null || !ref.current) return;
    caretAfter.current = null;
    ref.current.focus();
    ref.current.setSelectionRange(pos, pos);
  }, [value]);

  function track(el: HTMLTextAreaElement) {
    setMention(el.selectionStart === el.selectionEnd ? activeMention(el.value, el.selectionStart) : null);
  }

  function pick(u: MentionSuggestion) {
    const el = ref.current;
    if (!el || !mention) return;
    const insert = `@${u.handle} `;
    const next = value.slice(0, mention.start) + insert + value.slice(el.selectionStart);
    caretAfter.current = mention.start + insert.length;
    setMention(null);
    setUsers([]);
    onValueChange(next);
  }

  const open = mention !== null && users.length > 0;

  // Puts a placeholder at the caret for each image, uploads, then swaps the
  // placeholder for the image markdown (or removes it if the upload fails).
  function insertImages(files: File[]) {
    const images = files.filter(isImageFile);
    if (images.length === 0) return;
    const el = ref.current;
    const at = el ? el.selectionStart : latest.current.length;
    const tokens = images.map(
      (f) => `![${t("forum_image_uploading")} ${altFromFileName(f.name)}](#upload-${++uploadSeq})`,
    );
    const text = latest.current;
    const before = text.slice(0, at);
    const insert = (before && !before.endsWith("\n") ? "\n" : "") + tokens.join("\n") + "\n";
    caretAfter.current = at + insert.length;
    onValueChange(before + insert + text.slice(at));
    images.forEach((file, i) => {
      setUploading((n) => n + 1);
      uploadForumImage(file)
        .then((url) => onValueChange(latest.current.replace(tokens[i], `![${altFromFileName(file.name)}](${url})`)))
        .catch((e) => {
          onValueChange(latest.current.replace(tokens[i] + "\n", "").replace(tokens[i], ""));
          toast.error(forumErrorMessage(e));
        })
        .finally(() => setUploading((n) => n - 1));
    });
  }

  function keyDown(e: KeyboardEvent<HTMLTextAreaElement>) {
    if (open) {
      if (e.key === "ArrowDown" || e.key === "ArrowUp") {
        e.preventDefault();
        const step = e.key === "ArrowDown" ? 1 : -1;
        setIndex((i) => (i + step + users.length) % users.length);
        return;
      }
      if ((e.key === "Enter" && !e.metaKey && !e.ctrlKey) || e.key === "Tab") {
        e.preventDefault();
        pick(users[index]);
        return;
      }
      if (e.key === "Escape") {
        e.preventDefault();
        setMention(null);
        return;
      }
    }
    onKeyDown?.(e);
  }

  return (
    <div className="relative">
      <textarea
        {...rest}
        ref={ref}
        value={value}
        onPaste={(e) => {
          if (!allowImages) return;
          const files = Array.from(e.clipboardData.files);
          if (files.some(isImageFile)) {
            e.preventDefault();
            insertImages(files);
          }
        }}
        onDragOver={(e) => {
          if (allowImages && e.dataTransfer.types.includes("Files")) e.preventDefault();
        }}
        onDrop={(e) => {
          if (!allowImages || e.dataTransfer.files.length === 0) return;
          e.preventDefault();
          insertImages(Array.from(e.dataTransfer.files));
        }}
        onChange={(e) => {
          onValueChange(e.target.value);
          track(e.target);
        }}
        onKeyDown={keyDown}
        onKeyUp={(e) => {
          if (e.key.startsWith("Arrow") && !open) track(e.currentTarget);
        }}
        onClick={(e) => track(e.currentTarget)}
        onBlur={() => setTimeout(() => setMention(null), 150)}
        role="combobox"
        aria-expanded={open}
        aria-autocomplete="list"
        aria-controls={open ? listId : undefined}
        className={cn(className, allowImages && "pb-9")}
      />
      {allowImages && (
        <>
          <input
            ref={fileInput}
            type="file"
            accept="image/png,image/jpeg,image/webp,image/gif"
            multiple
            hidden
            onChange={(e) => {
              insertImages(Array.from(e.target.files ?? []));
              e.target.value = "";
            }}
          />
          <button
            type="button"
            onClick={() => fileInput.current?.click()}
            aria-label={t("forum_image_attach")}
            title={t("forum_image_attach_hint")}
            className="absolute bottom-2 right-2 grid size-7 place-items-center rounded-md text-muted hover:bg-surface-2 hover:text-ink"
          >
            {uploading > 0 ? <Loader2 size={16} className="animate-spin" /> : <ImagePlus size={16} />}
          </button>
        </>
      )}
      {open && (
        <ul
          id={listId}
          role="listbox"
          className="absolute left-2 top-full z-30 mt-1 w-72 max-w-[calc(100%-1rem)] overflow-hidden rounded-md border border-divider bg-surface py-1 shadow-lg"
        >
          {users.map((u, i) => (
            <li key={u.id} role="option" aria-selected={i === index}>
              <button
                type="button"
                // Keep focus in the textarea so blur doesn't close the list first.
                onMouseDown={(e) => e.preventDefault()}
                onClick={() => pick(u)}
                onMouseEnter={() => setIndex(i)}
                className={cn(
                  "flex w-full items-center gap-2 px-3 py-1.5 text-left text-sm",
                  i === index ? "bg-surface-2" : "",
                )}
              >
                <UserAvatar url={u.avatar_url} seed={u.handle} size={22} />
                <span className="min-w-0 truncate text-ink">{u.display_name || u.handle}</span>
                <span className="ml-auto shrink-0 truncate text-xs text-faint">@{u.handle}</span>
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
