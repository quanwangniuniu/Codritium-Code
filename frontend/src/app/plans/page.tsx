import { redirect } from "next/navigation";
import { Check } from "lucide-react";
import { currentUser } from "@/lib/auth";
import { t } from "@/shared/i18n";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";

const TIERS = [
  {
    key: "standard",
    name: "Standard",
    price: "Free",
    blurb: "Practice the open problem set with the full AI workspace.",
    perks: ["Open problem catalog", "AI agent in the editor", "5-dimension scoring", "Reply walkthroughs"],
  },
  {
    key: "pro",
    name: "Pro",
    price: "$19/mo",
    blurb: "Company-premium packs and unlimited graded runs.",
    perks: ["Everything in Standard", "Company premium packs", "Unlimited graded submissions", "Priority grading"],
  },
  {
    key: "max",
    name: "Max",
    price: "$49/mo",
    blurb: "For teams running structured interview practice at scale.",
    perks: ["Everything in Pro", "Team dashboards", "Custom problem imports", "Seat management"],
  },
] as const;

export default async function PlansPage() {
  const user = await currentUser();
  if (!user) redirect("/login");
  const currentTier = user.tier ?? "standard";

  return (
    <div className="mx-auto max-w-[1600px] px-6 py-10 space-y-6">
      <header>
        <h1 className="text-3xl tracking-tight font-semibold">{t("menu_plans")}</h1>
        <p className="text-sm text-muted mt-1">
          Pick the tier that matches how you practice. You are on{" "}
          <span className="text-ink font-medium capitalize">{currentTier}</span>.
        </p>
      </header>

      <div className="grid gap-4 sm:grid-cols-3">
        {TIERS.map((tier) => {
          const active = tier.key === currentTier;
          return (
            <Card
              key={tier.key}
              className={cn("flex h-full flex-col", active && "border-accent")}
            >
              <CardHeader>
                <div className="mb-1 flex items-center justify-between">
                  <CardTitle className="text-lg">{tier.name}</CardTitle>
                  {active && <Badge tone="info">Current</Badge>}
                </div>
                <div className="text-2xl font-semibold tracking-tight tabular-nums">
                  {tier.price}
                </div>
                <CardDescription className="mt-1 leading-relaxed">
                  {tier.blurb}
                </CardDescription>
              </CardHeader>
              <CardContent className="mt-auto">
                <ul className="space-y-2">
                  {tier.perks.map((p) => (
                    <li key={p} className="flex items-start gap-2 text-sm text-muted">
                      <Check size={15} className="mt-0.5 shrink-0 text-success" />
                      {p}
                    </li>
                  ))}
                </ul>
              </CardContent>
            </Card>
          );
        })}
      </div>

      <p className="text-xs text-faint">
        Billing is not wired up yet — tiers are illustrative for the launch preview.
      </p>
    </div>
  );
}
