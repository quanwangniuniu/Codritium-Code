// Monaco language id from a file name.
export function languageForFile(filename: string | null | undefined): string {
  if (!filename) return "plaintext";
  if (filename.endsWith(".py")) return "python";
  if (filename.endsWith(".ts") || filename.endsWith(".tsx")) return "typescript";
  if (filename.endsWith(".js") || filename.endsWith(".jsx")) return "javascript";
  if (filename.endsWith(".go")) return "go";
  if (filename.endsWith(".md")) return "markdown";
  return "plaintext";
}
