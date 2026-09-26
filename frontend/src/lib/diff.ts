// Tiny zero-dependency unified line diff used by PatchPreview.
//
// Implementation: classic LCS DP, then trace back. Quadratic in line count,
// which is fine for the source files V0 ships (the largest visible test
// is ~150 lines). If we ever break 5000 lines we'll swap to Myers' diff.

export type DiffOp = "=" | "-" | "+";

export interface DiffLine {
  op: DiffOp;
  text: string;
}

export function unifiedDiff(before: string, after: string): DiffLine[] {
  const a = before.split("\n");
  const b = after.split("\n");
  const m = a.length;
  const n = b.length;

  // dp[i][j] = LCS length of a[0..i-1] and b[0..j-1]
  const dp: number[][] = Array.from({ length: m + 1 }, () => new Array(n + 1).fill(0));
  for (let i = 1; i <= m; i++) {
    for (let j = 1; j <= n; j++) {
      if (a[i - 1] === b[j - 1]) {
        dp[i][j] = dp[i - 1][j - 1] + 1;
      } else {
        dp[i][j] = Math.max(dp[i - 1][j], dp[i][j - 1]);
      }
    }
  }

  // Trace back; build the diff in reverse, then reverse once.
  const out: DiffLine[] = [];
  let i = m;
  let j = n;
  while (i > 0 && j > 0) {
    if (a[i - 1] === b[j - 1]) {
      out.push({ op: "=", text: a[i - 1] });
      i--;
      j--;
    } else if (dp[i - 1][j] >= dp[i][j - 1]) {
      out.push({ op: "-", text: a[i - 1] });
      i--;
    } else {
      out.push({ op: "+", text: b[j - 1] });
      j--;
    }
  }
  while (i > 0) {
    out.push({ op: "-", text: a[i - 1] });
    i--;
  }
  while (j > 0) {
    out.push({ op: "+", text: b[j - 1] });
    j--;
  }
  return out.reverse();
}

// Compact stats helpful for the patch preview header.
export interface DiffStats {
  added: number;
  removed: number;
  unchanged: number;
}

export function diffStats(lines: DiffLine[]): DiffStats {
  const s: DiffStats = { added: 0, removed: 0, unchanged: 0 };
  for (const l of lines) {
    if (l.op === "+") s.added++;
    else if (l.op === "-") s.removed++;
    else s.unchanged++;
  }
  return s;
}
