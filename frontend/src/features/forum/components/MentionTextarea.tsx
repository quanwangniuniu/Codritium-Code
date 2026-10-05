"use client";

import { useEffect, useId, useRef, useState, type KeyboardEvent, type TextareaHTMLAttributes } from "react";
import { cn } from "@/shared/lib/cn";
import { UserAvatar } from "@/shared/avatar/UserAvatar";
import { forumApi, type MentionSuggestion } from "@/features/forum/api";
import { activeMention, type ActiveMention } from "@/features/forum/mentions";

interface MentionTextareaProps
  extends Omit<TextareaHTMLAttributes<HTMLTextAreaElement>, "value" | "onChange"> {
  value: string;
  onValueChange: (value: string) => void;
  // Ranks people already in this thread first.
  postId?: string;
}

// A textarea with @-mention autocomplete. Typing "@" offers people in the
// thread; typing more searches handles and display names. Arrow keys move,
// Enter or Tab picks, Escape dismisses.
export function MentionTextarea({ value, onValueChange, postId, onKeyDown, className, ...rest }: MentionTextareaProps) {
  const ref = useRef<HTMLTextAreaElement>(null);
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
        className={className}
      />
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
