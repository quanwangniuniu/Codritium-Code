import { redirect } from "next/navigation";
import { cookies } from "next/headers";
import { KeyRound, Info } from "lucide-react";
import { currentUser } from "@/lib/auth";
import { t } from "@/lib/i18n";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

const API_URL = process.env.CODRITIUM_API_URL ?? "http://localhost:8080";
const COOKIE_NAME = "codritium_session";

const MOCK_USERS = [
  { handle: "john", display_name: "John Smith", region: "AU" },
  { handle: "alice", display_name: "Alice Wang", region: "AU" },
  { handle: "bob", display_name: "Bob Martinez", region: "US" },
  { handle: "carol", display_name: "Carol Lee", region: "SG" },
  { handle: "dan", display_name: "Dan Patel", region: "IN" },
];

export default async function LoginPage() {
  const user = await currentUser();
  if (user) redirect("/problems");

  async function login(formData: FormData) {
    "use server";
    const handle = String(formData.get("handle") ?? "");
    const password = String(formData.get("password") ?? "");
    if (!handle || !password) {
      redirect("/login");
    }
    const res = await fetch(
      `${API_URL}/api/auth/switch?handle=${encodeURIComponent(handle)}`,
      { method: "POST" },
    );
    if (res.ok) {
      // Backend signs the cookie value (user_id + HMAC) and returns it via
      // Set-Cookie. The Next.js server has to relay that value to the
      // browser; without this the dev mock session cookie never reaches
      // localhost:3000, and every authed fetch comes back 401.
      const setCookie = res.headers.get("set-cookie") ?? "";
      const match = setCookie.match(/codritium_session=([^;]+)/);
      if (match) {
        const jar = await cookies();
        jar.set(COOKIE_NAME, match[1], {
          httpOnly: true,
          sameSite: "lax",
          path: "/",
          maxAge: 60 * 60 * 24 * 30,
        });
        redirect("/problems");
      }
    }
    redirect("/login");
  }

  return (
    <div className="mx-auto max-w-md px-6 py-16">
      <Card>
        <CardHeader>
          <CardTitle>{t("login_card_title")}</CardTitle>
          <CardDescription>{t("login_card_desc")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-5">
          <form action={login} className="space-y-4">
            <div className="space-y-2">
              <Label>{t("login_account_label")}</Label>
              <div className="space-y-2">
                {MOCK_USERS.map((u, idx) => (
                  <label
                    key={u.handle}
                    className="flex items-center gap-3 rounded-md border border-divider p-2 hover:bg-surface-2 cursor-pointer"
                  >
                    <input
                      type="radio"
                      name="handle"
                      value={u.handle}
                      defaultChecked={idx === 0}
                      className="accent-current"
                    />
                    <span className="text-sm">
                      <span className="font-medium">{u.display_name}</span>
                      <span className="text-faint"> · @{u.handle} · {u.region}</span>
                    </span>
                  </label>
                ))}
              </div>
            </div>

            <div className="space-y-2">
              <Label htmlFor="password">{t("password_placeholder")}</Label>
              <Input
                id="password"
                name="password"
                type="password"
                placeholder={t("login_password_placeholder_hint")}
                required
              />
            </div>

            <Button type="submit" size="lg" className="w-full">
              <KeyRound size={16} className="mr-2" />
              {t("sign_in")}
            </Button>
          </form>

          <div className="flex items-start gap-2 rounded-md border border-divider p-3 text-xs text-muted">
            <Info size={14} className="mt-0.5 flex-shrink-0" />
            <p>{t("login_footer_info")}</p>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
