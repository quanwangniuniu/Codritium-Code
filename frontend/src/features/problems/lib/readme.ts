// Problem READMEs open with a "# Title" line and a block of "**Key:** value"
// metadata lines (Company / Category / Difficulty). The problem page renders
// that information as its own header and chips, so this splits it off the
// body. The stored markdown is untouched; this is display-only.

export interface ProblemReadme {
  company: string | null;
  body: string;
}

const META_LINE = /^\*\*([^*]+):\*\*\s*(.*?)\s*$/;

export function splitProblemReadme(md: string): ProblemReadme {
  const lines = md.split("\n");
  let i = 0;
  while (i < lines.length && lines[i].trim() === "") i++;
  if (!lines[i]?.startsWith("# ")) return { company: null, body: md };
  i++;

  let company: string | null = null;
  while (i < lines.length) {
    const line = lines[i].trim();
    if (line === "") {
      i++;
      continue;
    }
    const m = META_LINE.exec(line);
    if (!m) break;
    if (m[1].trim().toLowerCase() === "company" && m[2]) company = m[2];
    i++;
  }
  return { company, body: lines.slice(i).join("\n") };
}
