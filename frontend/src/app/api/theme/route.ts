import { NextResponse } from "next/server";
import { setTheme, THEMES, type Theme } from "@/lib/theme";

export async function POST(request: Request) {
  const form = await request.formData();
  const theme = form.get("theme")?.toString();
  if (!theme || !(THEMES as readonly string[]).includes(theme)) {
    return NextResponse.json({ error: "Invalid theme." }, { status: 400 });
  }
  await setTheme(theme as Theme);
  const referer = request.headers.get("referer");
  const back = referer ? new URL(referer) : new URL("/", request.url);
  return NextResponse.redirect(back, { status: 303 });
}
