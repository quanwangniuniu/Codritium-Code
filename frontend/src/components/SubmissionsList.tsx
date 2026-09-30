"use client";

import { useMemo, useState } from "react";
import Link from "next/link";
import { CheckCircle2, ChevronLeft, ChevronRight, Trash2, X } from "lucide-react";
import {
  Card,
  CardContent,
  CardTitle,
} from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Backend } from "@/lib/api";
import { formatDateTime } from "@/shared/format";
import { toast } from "@/lib/toast";
import { t as tr } from "@/lib/i18n";
import { useLocale } from "@/lib/i18n-client";
import type { Problem, Submission } from "@/lib/types";

const PAGE_SIZE = 8;

interface Props {
  initialSubmissions: Submission[];
  problemMap: Record<string, Problem>;
}

export function SubmissionsList({ initialSubmissions, problemMap }: Props) {
  useLocale();
  const [submissions, setSubmissions] = useState<Submission[]>(initialSubmissions);
  const [selectMode, setSelectMode] = useState(false);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [page, setPage] = useState(1);
  const [deleting, setDeleting] = useState(false);

  const totalPages = Math.max(1, Math.ceil(submissions.length / PAGE_SIZE));
  const safePage = Math.min(page, totalPages);
  const startIdx = (safePage - 1) * PAGE_SIZE;
  const pageRows = useMemo(
    () => submissions.slice(startIdx, startIdx + PAGE_SIZE),
    [submissions, startIdx],
  );

  const toggleSelectMode = () => {
    setSelectMode((v) => {
      if (v) setSelected(new Set());
      return !v;
    });
  };

  const toggleSelected = (id: string) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const handleDelete = async () => {
    if (selected.size === 0 || deleting) return;
    setDeleting(true);
    const ids = [...selected];
    const failures: string[] = [];
    for (const id of ids) {
      try {
        await Backend.deleteSubmission(id);
      } catch (e) {
        console.error("delete failed:", id, e);
        failures.push(id);
      }
    }
    const succeeded = ids.filter((id) => !failures.includes(id));
    setSubmissions((prev) => prev.filter((s) => !succeeded.includes(s.id)));
    setSelected(new Set());
    setDeleting(false);
    if (failures.length === 0) {
      toast.success(tr("submissions_deleted"));
      setSelectMode(false);
    } else {
      toast.warn(tr("submissions_delete_partial"));
    }
  };

  if (submissions.length === 0) {
    return (
      <Card>
        <CardContent className="py-10 text-center text-sm text-muted">
          {tr("submissions_empty_prefix")}{" "}
          <Link href="/problems" className="underline text-ink">
            {tr("submissions_problem_list_word")}
          </Link>
          .
        </CardContent>
      </Card>
    );
  }

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between gap-2">
        <div className="text-xs text-muted">
          {tr("submissions_total_fmt", { params: { total: submissions.length, page: safePage, pages: totalPages } })}
        </div>
        <div className="flex items-center gap-2">
          {selectMode ? (
            <>
              <Button
                onClick={handleDelete}
                disabled={selected.size === 0 || deleting}
                size="sm"
                variant="outline"
                className="gap-1.5"
              >
                <Trash2 size={14} strokeWidth={1.7} />
                {selected.size > 0
                  ? tr("submissions_delete_btn_count_fmt", { params: { n: selected.size } })
                  : tr("submissions_delete_btn")}
              </Button>
              <Button
                onClick={toggleSelectMode}
                size="sm"
                variant="ghost"
                className="gap-1.5"
              >
                <X size={14} strokeWidth={1.7} />
                {tr("cancel")}
              </Button>
            </>
          ) : (
            <Button
              onClick={toggleSelectMode}
              size="sm"
              variant="outline"
              className="gap-1.5"
            >
              <CheckCircle2 size={14} strokeWidth={1.7} />
              {tr("submissions_select_btn")}
            </Button>
          )}
        </div>
      </div>

      <div className="space-y-3">
        {pageRows.map((s) => {
          const problem = problemMap[s.problem_id] ?? null;
          const total = s.score?.total ?? 0;
          const isSelected = selected.has(s.id);
          return (
            <SubmissionRow
              key={s.id}
              submission={s}
              total={total}
              title={problem?.title ?? tr("submissions_unknown_problem")}
              difficulty={problem?.difficulty ?? null}
              selectMode={selectMode}
              isSelected={isSelected}
              onToggle={() => toggleSelected(s.id)}
            />
          );
        })}
      </div>

      <div className="flex items-center justify-between pt-2">
        <Button
          onClick={() => setPage((p) => Math.max(1, p - 1))}
          disabled={safePage === 1}
          size="sm"
          variant="outline"
          className="gap-1.5"
        >
          <ChevronLeft size={14} strokeWidth={1.7} />
          {tr("nav_prev")}
        </Button>
        <div className="text-xs text-muted tabular-nums">
          {safePage} / {totalPages}
        </div>
        <Button
          onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
          disabled={safePage >= totalPages}
          size="sm"
          variant="outline"
          className="gap-1.5"
        >
          {tr("nav_next")}
          <ChevronRight size={14} strokeWidth={1.7} />
        </Button>
      </div>
    </div>
  );
}

function SubmissionRow({
  submission,
  total,
  title,
  difficulty,
  selectMode,
  isSelected,
  onToggle,
}: {
  submission: Submission;
  total: number;
  title: string;
  difficulty: "easy" | "medium" | "hard" | null;
  selectMode: boolean;
  isSelected: boolean;
  onToggle: () => void;
}) {
  const body = (
    <Card
      className={
        "transition-colors " +
        (selectMode
          ? isSelected
            ? "border-accent bg-accent-soft"
            : "hover:border-divider-strong"
          : "group-hover:border-divider-strong")
      }
    >
      <CardContent className="py-4">
        <div className="flex items-center gap-4">
          {selectMode && (
            <input
              type="checkbox"
              checked={isSelected}
              onChange={onToggle}
              onClick={(e) => e.stopPropagation()}
              className="h-4 w-4 accent-current cursor-pointer"
              aria-label={tr("submissions_select_row_aria_fmt", { params: { title } })}
            />
          )}
          <div className="text-2xl font-semibold tabular-nums w-16 tracking-tight text-ink">
            {total.toFixed(0)}
          </div>
          <div className="flex-1 min-w-0">
            <CardTitle className="text-sm truncate">{title}</CardTitle>
            <p className="text-xs text-muted mt-0.5">
              {formatDateTime(submission.submitted_at)}
              {submission.status !== "completed" && (
                <span className="ml-1">· {submission.status}</span>
              )}
            </p>
          </div>
          {difficulty && (
            <Badge
              tone={
                difficulty === "easy"
                  ? "success"
                  : difficulty === "medium"
                    ? "warning"
                    : "danger"
              }
            >
              {difficulty}
            </Badge>
          )}
        </div>
      </CardContent>
    </Card>
  );

  if (selectMode) {
    // In select mode the whole row toggles selection; no navigation.
    return (
      <div
        onClick={onToggle}
        role="button"
        className="cursor-pointer block"
      >
        {body}
      </div>
    );
  }

  return (
    <Link href={`/submissions/${submission.id}`} className="group block">
      {body}
    </Link>
  );
}
