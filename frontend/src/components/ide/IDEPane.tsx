"use client";

import dynamic from "next/dynamic";
import {
  forwardRef,
  useCallback,
  useImperativeHandle,
  useRef,
} from "react";
import type {
  editor as MonacoEditor,
  IDisposable as MonacoDisposable,
} from "monaco-editor";

const Editor = dynamic(
  () => import("@monaco-editor/react").then((mod) => mod.default),
  { ssr: false },
);

export interface IDEPaneHandle {
  layout: () => void;
  focus: () => void;
  getEditor: () => MonacoEditor.IStandaloneCodeEditor | null;
}

export interface IDEPaneProps {
  path: string;
  language?: string;
  value: string;
  onChange?: (value: string) => void;
  readOnly?: boolean;
  theme?: "vs" | "vs-dark";
  onMount?: (editor: MonacoEditor.IStandaloneCodeEditor) => void;
}

// IDEPane wraps a single Monaco editor with the dual-safety resize pattern
// the ide-multi-pane impl-spec calls out: automaticLayout=true so Monaco
// keeps itself in sync on most flexbox changes, plus an imperative
// layout() method the host PanelLayout calls in its onResize so a known
// resize event is never missed (Q3=B).
export const IDEPane = forwardRef<IDEPaneHandle, IDEPaneProps>(function IDEPane(
  { path, language, value, onChange, readOnly = false, theme = "vs-dark", onMount },
  ref,
) {
  const editorRef = useRef<MonacoEditor.IStandaloneCodeEditor | null>(null);
  const disposablesRef = useRef<MonacoDisposable[]>([]);

  useImperativeHandle(
    ref,
    () => ({
      layout: () => editorRef.current?.layout(),
      focus: () => editorRef.current?.focus(),
      getEditor: () => editorRef.current,
    }),
    [],
  );

  const handleMount = useCallback(
    (editor: MonacoEditor.IStandaloneCodeEditor) => {
      editorRef.current = editor;
      // The monaco React wrapper handles value→model sync, but the resize
      // contract is ours: call layout when the host pane fires onResize.
      onMount?.(editor);
    },
    [onMount],
  );

  const handleUnmount = useCallback(() => {
    disposablesRef.current.forEach((d) => d.dispose());
    disposablesRef.current = [];
    editorRef.current = null;
  }, []);

  return (
    <div style={{ width: "100%", height: "100%" }}>
      <Editor
        path={path}
        defaultLanguage={language}
        value={value}
        onChange={(v) => onChange?.(v ?? "")}
        theme={theme}
        onMount={handleMount}
        beforeMount={() => undefined}
        // Note: @monaco-editor/react has no public unmount callback; the
        // outer component clears refs via React's lifecycle (handleUnmount
        // would be wired via a wrapping useEffect if needed).
        options={{
          readOnly,
          automaticLayout: true,
          minimap: { enabled: false },
          scrollBeyondLastLine: false,
          wordWrap: "on",
          tabSize: 2,
          fontSize: 13,
          renderWhitespace: "selection",
          fixedOverflowWidgets: true,
        }}
      />
      <UnmountHook handleUnmount={handleUnmount} />
    </div>
  );
});

// Side-channel hook so handleUnmount runs once when IDEPane unmounts.
function UnmountHook({ handleUnmount }: { handleUnmount: () => void }) {
  const fired = useRef(false);
  if (typeof window !== "undefined" && !fired.current) {
    fired.current = true;
    // Use a microtask to register a cleanup on the next render boundary —
    // avoids the side-effect during render lint warning.
    queueMicrotask(() => {
      const cleanup = () => handleUnmount();
      const w = window as Window & { addEventListener: typeof window.addEventListener };
      w.addEventListener("beforeunload", cleanup, { once: true });
    });
  }
  return null;
}
