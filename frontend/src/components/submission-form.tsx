"use client";

import { useRouter } from "next/navigation";
import { useState, useTransition } from "react";
import { Send, AlertCircle } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import { t } from "@/lib/i18n";
import { useLocale } from "@/lib/i18n-client";
import type { AiTool, Problem } from "@/lib/types";

interface SubmissionFormProps {
  problem: Problem;
}

export function SubmissionForm({ problem }: SubmissionFormProps) {
  useLocale();
  const router = useRouter();
  const [pending, startTransition] = useTransition();

  const [files, setFiles] = useState<Record<string, string>>({ ...problem.starter_files });
  const [activeFile, setActiveFile] = useState<string>(Object.keys(problem.starter_files)[0]);
  const [promptHistory, setPromptHistory] = useState<string>("");
  const [aiTool, setAiTool] = useState<AiTool>("claude_code");
  const [aiModel, setAiModel] = useState<string>("");
  const [numPrompts, setNumPrompts] = useState<number>(3);
  const [error, setError] = useState<string | null>(null);

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    startTransition(async () => {
      const res = await fetch("/api/grade", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          problem_id: problem.id,
          submitted_files: files,
          prompt_history: promptHistory,
          ai_tool_used: aiTool,
          ai_model_used: aiModel,
          num_prompts: numPrompts,
        }),
      });
      if (!res.ok) {
        const body = await res.json().catch(() => ({}));
        setError(body.error ?? t("subform_submission_failed_fallback"));
        return;
      }
      const { submission_id } = await res.json();
      router.push(`/submissions/${submission_id}`);
    });
  }

  const filenames = Object.keys(files);

  return (
    <form onSubmit={handleSubmit} className="space-y-6">
      <div className="space-y-2">
        <Label>{t("subform_files_label")}</Label>
        <div className="flex gap-2 border-b border-divider">
          {filenames.map((fn) => (
            <button
              key={fn}
              type="button"
              onClick={() => setActiveFile(fn)}
              className={cn(
                "px-3 py-1.5 text-xs font-mono border-b-2 -mb-px transition-colors",
                activeFile === fn
                  ? "border-accent text-ink"
                  : "border-transparent text-muted hover:text-ink",
              )}
            >
              {fn}
            </button>
          ))}
        </div>
        <Textarea
          value={files[activeFile] ?? ""}
          onChange={(e) => setFiles({ ...files, [activeFile]: e.target.value })}
          rows={14}
          spellCheck={false}
          className="text-xs"
        />
      </div>

      <div className="grid sm:grid-cols-3 gap-4">
        <div className="space-y-2">
          <Label htmlFor="ai_tool">{t("subform_ai_tool_label")}</Label>
          <Select id="ai_tool" value={aiTool} onChange={(e) => setAiTool(e.target.value as AiTool)}>
            <option value="claude_code">{t("ai_tool_claude_code")}</option>
            <option value="cursor">{t("ai_tool_cursor")}</option>
            <option value="copilot">{t("ai_tool_copilot")}</option>
            <option value="chatgpt">{t("ai_tool_chatgpt")}</option>
            <option value="claude_app">{t("ai_tool_claude_app")}</option>
            <option value="other">{t("ai_tool_other")}</option>
          </Select>
        </div>
        <div className="space-y-2">
          <Label htmlFor="ai_model">{t("subform_model_label")}</Label>
          <Input
            id="ai_model"
            value={aiModel}
            onChange={(e) => setAiModel(e.target.value)}
            placeholder={t("subform_model_placeholder")}
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor="num_prompts">{t("subform_num_prompts_label")}</Label>
          <Input
            id="num_prompts"
            type="number"
            min={0}
            value={numPrompts}
            onChange={(e) => setNumPrompts(Number(e.target.value))}
          />
        </div>
      </div>

      <div className="space-y-2">
        <Label htmlFor="prompt_history">{t("subform_prompt_history_label")}</Label>
        <p className="text-xs text-muted">{t("subform_prompt_history_blurb")}</p>
        <Textarea
          id="prompt_history"
          rows={8}
          value={promptHistory}
          onChange={(e) => setPromptHistory(e.target.value)}
          placeholder={t("subform_prompt_history_placeholder")}
        />
      </div>

      {error && (
        <div className="flex items-center gap-2 rounded-md border border-danger/30 bg-danger-soft px-3 py-2 text-sm text-danger">
          <AlertCircle size={14} />
          {error}
        </div>
      )}

      <div className="flex items-center justify-end gap-2">
        <Button type="submit" disabled={pending}>
          <Send size={14} className="mr-1.5" />
          {pending ? t("subform_submitting") : t("subform_submit_btn")}
        </Button>
      </div>
    </form>
  );
}
