import { Check, FileCode2, Sparkles, X } from "lucide-react";
import { t, type LocaleKey } from "@/shared/i18n";
import { cn } from "@/shared/lib/cn";

// Static illustration of the workspace for the hero: an editor with an AI
// patch applied, the agent chat, and a sample score card. Decorative only.

type Tok = [string, string?];
type Line = { kind?: "add" | "del"; toks: Tok[] };

const KW = "text-home-blue";
const STR = "text-home-amber";
const FN = "text-home-teal";
const CMT = "text-faint";

const CODE: Line[] = [
  { toks: [["def ", KW], ["handle_checkout_completed", FN], ["(event, ledger):"]] },
  { toks: [["    session = event["], ['"data"', STR], ["]["], ['"object"', STR], ["]"]] },
  { toks: [["    # Stripe retries reuse the same event id", CMT]] },
  { kind: "del", toks: [["    ledger.dispatch(session["], ['"id"', STR], ["])"]] },
  { kind: "add", toks: [["    ", undefined], ["if ", KW], ["ledger.seen(event["], ['"id"', STR], ["]):"]] },
  { kind: "add", toks: [["        ", undefined], ["return ", KW], ["ledger.skip_replay(event["], ['"id"', STR], ["])"]] },
  { kind: "add", toks: [["    ledger.dispatch(session["], ['"id"', STR], ["], event_id=event["], ['"id"', STR], ["])"]] },
  { toks: [["    ", undefined], ["return ", KW], ["ledger.summary()"]] },
];

const SCORES: { key: LocaleKey; value: number }[] = [
  { key: "dim_label_correctness", value: 92 },
  { key: "dim_label_decomposition", value: 84 },
  { key: "dim_label_ai_collab", value: 88 },
  { key: "dim_label_verification", value: 76 },
  { key: "dim_label_communication", value: 81 },
];

export function ProductMock() {
  return (
    <div aria-hidden className="relative select-none">
      <div className="home-window overflow-hidden -rotate-1 lg:-rotate-2">
        <div className="flex items-center gap-2 px-4 h-10 border-b border-divider bg-surface-2/60">
          <span className="size-2.5 rounded-full bg-home-rose/70" />
          <span className="size-2.5 rounded-full bg-home-amber/70" />
          <span className="size-2.5 rounded-full bg-home-green/70" />
          <span className="ml-3 flex items-center gap-1.5 text-xs text-muted mono">
            <FileCode2 size={13} />
            checkout_fulfillment.py
          </span>
        </div>
        <div className="grid sm:grid-cols-[1fr_210px]">
          <pre className="mono text-[11.5px] leading-[1.9] py-3 overflow-hidden">
            {CODE.map((line, i) => (
              <div
                key={i}
                className={cn(
                  "flex pr-3",
                  line.kind === "add" && "bg-home-green-soft",
                  line.kind === "del" && "bg-home-rose-soft line-through decoration-home-rose/50",
                )}
              >
                <span className="w-9 shrink-0 text-right pr-3 text-faint">{i + 1}</span>
                <span className="w-3 shrink-0 text-faint">
                  {line.kind === "add" ? "+" : line.kind === "del" ? "-" : ""}
                </span>
                <code className="whitespace-pre">
                  {line.toks.map(([text, cls], j) => (
                    <span key={j} className={cls}>
                      {text}
                    </span>
                  ))}
                </code>
              </div>
            ))}
          </pre>
          <div className="hidden sm:flex flex-col gap-2.5 border-l border-divider p-3 text-[11.5px] leading-snug bg-canvas/50">
            <div className="self-end max-w-[90%] rounded-xl rounded-br-sm bg-accent text-accent-fg px-2.5 py-1.5">
              {t("home_mock_user_msg")}
            </div>
            <div className="flex gap-1.5 max-w-[95%]">
              <Sparkles size={13} className="text-home-teal shrink-0 mt-0.5" />
              <div className="rounded-xl rounded-tl-sm bg-surface-2 px-2.5 py-1.5 text-ink">
                {t("home_mock_ai_msg")}
              </div>
            </div>
            <div className="mt-auto rounded-lg border border-divider bg-surface p-2">
              <p className="text-[10.5px] uppercase tracking-wider text-faint mb-1.5">
                {t("home_mock_patch")}
              </p>
              <div className="flex gap-1.5">
                <span className="flex-1 inline-flex items-center justify-center gap-1 rounded-md bg-home-green text-white py-1">
                  <Check size={11} /> {t("home_mock_approve")}
                </span>
                <span className="flex-1 inline-flex items-center justify-center gap-1 rounded-md border border-divider text-muted py-1">
                  <X size={11} /> {t("home_mock_reject")}
                </span>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div className="home-window absolute -bottom-40 -left-6 lg:-left-12 w-[230px] p-3.5 rotate-2 hidden sm:block">
        <div className="flex items-baseline justify-between mb-2.5">
          <p className="text-[11px] uppercase tracking-wider text-faint">{t("home_mock_report")}</p>
          <p className="text-lg font-semibold text-ink">84</p>
        </div>
        <div className="space-y-2">
          {SCORES.map((s) => (
            <div key={s.key}>
              <div className="flex justify-between text-[10.5px] text-muted mb-0.5">
                <span className="truncate">{t(s.key)}</span>
                <span>{s.value}</span>
              </div>
              <div className="h-1.5 rounded-full bg-surface-2 overflow-hidden">
                <div className="h-full rounded-full bg-home-blue" style={{ width: `${s.value}%` }} />
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
