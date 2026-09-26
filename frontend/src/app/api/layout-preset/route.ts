import { NextResponse } from "next/server";
import {
  setLayoutPreset,
  LAYOUT_PRESETS,
  type LayoutPreset,
} from "@/lib/layout-preset";

export async function POST(request: Request) {
  const form = await request.formData();
  const preset = form.get("preset")?.toString();
  if (!preset || !(LAYOUT_PRESETS as readonly string[]).includes(preset)) {
    return NextResponse.json(
      { error: "Invalid layout preset." },
      { status: 400 },
    );
  }
  await setLayoutPreset(preset as LayoutPreset);
  const referer = request.headers.get("referer");
  const back = referer ? new URL(referer) : new URL("/", request.url);
  return NextResponse.redirect(back, { status: 303 });
}
