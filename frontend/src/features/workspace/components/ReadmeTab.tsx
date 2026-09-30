"use client";

import { ExternalLink, Minimize2 } from "lucide-react";
import ReactMarkdown from "react-markdown";
import { t } from "@/shared/i18n";

export function ReadmeBody({ readme }: { readme: string }) {
  return (
    <div
      style={{
        fontSize: 14,
        lineHeight: 1.7,
        color: "var(--text)",
        maxWidth: 760,
        margin: "0 auto",
      }}
    >
      <ReactMarkdown
        components={{
          h1: ({ children }) => (
            <h1
              style={{
                fontSize: 22,
                color: "var(--text-strong)",
                fontWeight: 600,
                margin: "8px 0 18px",
                paddingBottom: 10,
                borderBottom: "1px solid var(--border)",
                letterSpacing: "-0.01em",
              }}
            >
              {children}
            </h1>
          ),
          h2: ({ children }) => (
            <h2
              style={{
                fontSize: 16,
                color: "var(--text-strong)",
                fontWeight: 600,
                marginTop: 26,
                marginBottom: 10,
                letterSpacing: "-0.005em",
              }}
            >
              {children}
            </h2>
          ),
          h3: ({ children }) => (
            <h3
              style={{
                fontSize: 14,
                color: "var(--text-strong)",
                fontWeight: 600,
                marginTop: 18,
                marginBottom: 8,
              }}
            >
              {children}
            </h3>
          ),
          p: ({ children }) => (
            <p style={{ margin: "10px 0", color: "var(--text)" }}>{children}</p>
          ),
          strong: ({ children }) => (
            <strong style={{ color: "var(--text-strong)", fontWeight: 600 }}>
              {children}
            </strong>
          ),
          em: ({ children }) => (
            <em style={{ color: "var(--text)", fontStyle: "italic" }}>
              {children}
            </em>
          ),
          ul: ({ children }) => (
            <ul
              style={{
                paddingLeft: 24,
                margin: "8px 0 14px",
                listStyleType: "disc",
              }}
            >
              {children}
            </ul>
          ),
          ol: ({ children }) => (
            <ol
              style={{
                paddingLeft: 24,
                margin: "8px 0 14px",
                listStyleType: "decimal",
              }}
            >
              {children}
            </ol>
          ),
          li: ({ children }) => (
            <li style={{ margin: "4px 0", color: "var(--text)" }}>{children}</li>
          ),
          a: ({ children, href }) => (
            <a
              href={href}
              style={{
                color: "var(--accent)",
                textDecoration: "underline",
                textDecorationColor: "rgba(78, 168, 222, 0.4)",
                textUnderlineOffset: 2,
              }}
              target="_blank"
              rel="noreferrer"
            >
              {children}
            </a>
          ),
          code: ({ children }) => (
            <code
              className="mono"
              style={{
                background: "var(--bg-tab)",
                color: "var(--text-strong)",
                padding: "1px 6px",
                borderRadius: 3,
                fontSize: 12.5,
                border: "1px solid var(--border)",
              }}
            >
              {children}
            </code>
          ),
          pre: ({ children }) => (
            <pre
              className="mono"
              style={{
                background: "var(--bg-side)",
                color: "var(--text)",
                padding: "12px 14px",
                borderRadius: 6,
                border: "1px solid var(--border)",
                margin: "12px 0",
                fontSize: 12.5,
                overflowX: "auto",
                lineHeight: 1.55,
              }}
            >
              {children}
            </pre>
          ),
          blockquote: ({ children }) => (
            <blockquote
              style={{
                margin: "12px 0",
                paddingLeft: 14,
                borderLeft: "3px solid var(--border)",
                color: "var(--text-dim)",
                fontStyle: "italic",
              }}
            >
              {children}
            </blockquote>
          ),
          hr: () => (
            <hr
              style={{
                border: "none",
                borderTop: "1px solid var(--border)",
                margin: "20px 0",
              }}
            />
          ),
        }}
      >
        {readme}
      </ReactMarkdown>
    </div>
  );
}

export function ReadmeTab({
  readme,
  floating,
  onToggleFloat,
}: {
  readme: string;
  floating: boolean;
  onToggleFloat: () => void;
}) {
  return (
    <div
      style={{
        height: "100%",
        display: "flex",
        flexDirection: "column",
        background: "var(--bg-editor)",
      }}
    >
      <div
        style={{
          display: "flex",
          justifyContent: "flex-end",
          padding: "6px 14px",
          borderBottom: "1px solid var(--border)",
          flexShrink: 0,
        }}
      >
        <button
          onClick={onToggleFloat}
          title={floating ? t("readme_dock_btn") : t("readme_float_btn")}
          style={{
            display: "flex",
            alignItems: "center",
            gap: 5,
            padding: "4px 10px",
            fontSize: 11.5,
            color: "var(--text)",
            background: "var(--bg-tab)",
            border: "1px solid var(--border)",
            borderRadius: 3,
            cursor: "pointer",
          }}
        >
          {floating ? (
            <Minimize2 size={12} strokeWidth={1.6} />
          ) : (
            <ExternalLink size={12} strokeWidth={1.6} />
          )}
          <span>{floating ? t("readme_dock_btn") : t("readme_float_btn")}</span>
        </button>
      </div>
      <div
        className="scroll-y"
        style={{
          flex: 1,
          overflow: "auto",
          padding: "28px 40px",
        }}
      >
        {floating ? (
          <div
            style={{
              display: "flex",
              flexDirection: "column",
              alignItems: "center",
              justifyContent: "center",
              gap: 14,
              height: "100%",
              color: "var(--text-muted)",
              fontSize: 13,
              textAlign: "center",
              userSelect: "none",
            }}
          >
            <ExternalLink size={36} strokeWidth={1} style={{ opacity: 0.4 }} />
            <div
              style={{
                fontSize: 13,
                fontWeight: 500,
                color: "var(--text-dim)",
              }}
            >
              {t("readme_floating_title")}
            </div>
            <div style={{ fontSize: 11.5, lineHeight: 1.7, maxWidth: 320 }}>
              {t("readme_floating_hint")}
            </div>
          </div>
        ) : (
          <ReadmeBody readme={readme} />
        )}
      </div>
    </div>
  );
}
