"use client";

import { useEffect, useRef, useState } from "react";

export interface ComposerOptions {
  // While busy, submitted messages are queued and sent one by one as soon
  // as busy flips back to false.
  busy: boolean;
  onSend: (text: string) => void;
  // Candidates for @file mentions.
  files?: string[];
  // Blocks submitting (e.g. after the solution was submitted).
  disabled?: boolean;
  // Runs on every accepted submit, before send / queue.
  onSubmitted?: () => void;
}

export type Composer = ReturnType<typeof useComposer>;

const MAX_MENTION_MATCHES = 6;

// Input state shared by the agent chat and the tutor: text, the send
// queue, and @file mention autocomplete.
export function useComposer({ busy, onSend, files, disabled, onSubmitted }: ComposerOptions) {
  const [text, setText] = useState("");
  const [queue, setQueue] = useState<string[]>([]);
  const [mention, setMention] = useState<{ query: string; start: number } | null>(null);
  const prevBusyRef = useRef(busy);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  const mentionMatches = (() => {
    if (!mention || !files) return [];
    const q = mention.query.toLowerCase();
    return files.filter((f) => f.toLowerCase().includes(q)).slice(0, MAX_MENTION_MATCHES);
  })();
  const mentionOpen = mention !== null && mentionMatches.length > 0;

  // Auto-dispatch queued message when the agent transitions from
  // busy → idle, so Enter-during-streaming behaves like a queue.
  useEffect(() => {
    const wasBusy = prevBusyRef.current;
    prevBusyRef.current = busy;
    if (wasBusy && !busy && queue.length > 0) {
      const next = queue[0];
      setQueue((q) => q.slice(1));
      onSend(next);
    }
  }, [busy, queue, onSend]);

  const submit = () => {
    const value = text.trim();
    if (!value || disabled) return;
    setText("");
    setMention(null);
    onSubmitted?.();
    if (busy) {
      setQueue((q) => [...q, value]);
      return;
    }
    onSend(value);
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    submit();
  };

  const handleChange = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
    const next = e.target.value;
    setText(next);
    const cursor = e.target.selectionStart ?? next.length;
    const m = next.slice(0, cursor).match(/@(\S*)$/);
    setMention(m ? { query: m[1], start: cursor - m[0].length } : null);
  };

  const selectMention = (filename: string) => {
    if (!mention) return;
    const before = text.slice(0, mention.start);
    const after = text.slice(textareaRef.current?.selectionStart ?? text.length);
    setText(`${before}@${filename} ${after}`);
    setMention(null);
    requestAnimationFrame(() => {
      const ta = textareaRef.current;
      if (ta) {
        const pos = before.length + filename.length + 2; // @ + name + space
        ta.focus();
        ta.setSelectionRange(pos, pos);
      }
    });
  };

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (mentionOpen) {
      if (e.key === "Tab" || (e.key === "Enter" && !e.shiftKey)) {
        e.preventDefault();
        selectMention(mentionMatches[0]);
        return;
      }
      if (e.key === "Escape") {
        e.preventDefault();
        setMention(null);
        return;
      }
    }
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      submit();
    }
  };

  return {
    text,
    queue,
    mentionOpen,
    mentionMatches,
    textareaRef,
    canSubmit: text.trim().length > 0 && !disabled,
    handleSubmit,
    handleChange,
    handleKeyDown,
    selectMention,
  };
}
