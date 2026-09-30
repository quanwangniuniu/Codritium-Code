import { redirect } from "next/navigation";
import { cookies } from "next/headers";
import Link from "next/link";
import { KeyRound, Info } from "lucide-react";
import { currentUser } from "@/lib/auth";
import { t } from "@/shared/i18n";
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

interface LoginPageProps {
  searchParams: Promise<{ mode?: string; error?: string; email?: string }>;
}

export default async function LoginPage({ searchParams }: LoginPageProps) {
  const params = await searchParams;
  const isRegister = params.mode === "register";
  const user = await currentUser();
  if (user) redirect("/problems");

  async function authenticate(formData: FormData) {
    "use server";
    const email = String(formData.get("email") ?? "").trim();
    const password = String(formData.get("password") ?? "");
    const mode = isRegister ? "register" : "login";
    let response: Response | undefined;
    try {
      response = await fetch(`${API_URL}/api/auth/${mode}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email, password }),
      });
    } catch {
      // Show a form-level error if the API is unavailable.
    }

    if (response?.ok) {
      const setCookie = response.headers.get("set-cookie") ?? "";
      const pair = setCookie.split(";", 1)[0];
      const separator = pair.indexOf("=");
      if (separator >= 0 && pair.slice(0, separator) === COOKIE_NAME) {
        const jar = await cookies();
        jar.set(COOKIE_NAME, pair.slice(separator + 1), {
          httpOnly: true,
          sameSite: "lax",
          path: "/",
          maxAge: 60 * 60 * 24 * 30,
          secure: process.env.NODE_ENV === "production",
        });
        redirect("/problems");
      }
    }

    const error =
      isRegister && response?.status === 409
        ? "email_exists"
        : !isRegister && response?.status === 401
          ? "invalid_credentials"
          : response?.status === 400
            ? "invalid_input"
            : "request_failed";
    const query = new URLSearchParams({ mode, error, email });
    redirect(`/login?${query.toString()}`);
  }

  const errorMessage =
    params.error === "email_exists"
      ? t("auth_email_exists")
      : params.error === "invalid_credentials"
        ? t("auth_invalid_credentials")
        : params.error === "invalid_input"
          ? t("auth_invalid_input")
          : params.error
            ? t("auth_request_failed")
            : "";

  return (
    <div className="mx-auto max-w-md px-6 py-16">
      <Card>
        <CardHeader>
          <CardTitle>{isRegister ? t("register_card_title") : t("login_card_title")}</CardTitle>
          <CardDescription>{isRegister ? t("register_card_desc") : t("login_card_desc")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-5">
          <form action={authenticate} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="email">{t("login_account_label")}</Label>
              <Input
                id="email"
                name="email"
                type="email"
                autoComplete="email"
                defaultValue={params.email ?? ""}
                required
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="password">{t("password_placeholder")}</Label>
              <Input
                id="password"
                name="password"
                type="password"
                autoComplete={isRegister ? "new-password" : "current-password"}
                minLength={isRegister ? 8 : undefined}
                maxLength={72}
                placeholder={isRegister ? t("register_password_hint") : ""}
                required
              />
            </div>

            <Button type="submit" size="lg" className="w-full">
              <KeyRound size={16} className="mr-2" />
              {isRegister ? t("create_account") : t("sign_in")}
            </Button>
          </form>

          {errorMessage && (
            <p role="alert" className="text-sm text-destructive">
              {errorMessage}
            </p>
          )}

          <p className="text-center text-sm text-muted">
            {isRegister ? t("login_prompt") : t("register_prompt")} {" "}
            <Link
              href={isRegister ? "/login" : "/login?mode=register"}
              className="font-medium text-ink underline underline-offset-4"
            >
              {isRegister ? t("sign_in") : t("create_account")}
            </Link>
          </p>

          <div className="flex items-start gap-2 rounded-md border border-divider p-3 text-xs text-muted">
            <Info size={14} className="mt-0.5 flex-shrink-0" />
            <p>{t("login_footer_info")}</p>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
