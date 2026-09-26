import Link from "next/link";
import { Calendar, Sparkles } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import type { Problem } from "@/lib/types";

interface DailyChallengeCardProps {
  problem: Problem;
  date: string;
}

export function DailyChallengeCard({ problem, date }: DailyChallengeCardProps) {
  return (
    <Card className="overflow-hidden">
      <div className="bg-accent-soft text-accent px-5 py-2 flex items-center gap-2 text-xs font-medium">
        <Sparkles size={14} />
        Daily Challenge
        <span className="ml-auto inline-flex items-center gap-1 opacity-80">
          <Calendar size={12} />
          {date}
        </span>
      </div>
      <CardHeader className="pt-4">
        <div className="flex items-center gap-2 mb-2">
          <Badge tone="info">{problem.category}</Badge>
          <Badge
            tone={
              problem.difficulty === "easy"
                ? "success"
                : problem.difficulty === "medium"
                  ? "warning"
                  : "danger"
            }
          >
            {problem.difficulty}
          </Badge>
          {problem.ray_seed && <Badge tone="neutral">private source</Badge>}
        </div>
        <CardTitle>{problem.title}</CardTitle>
        <CardDescription>
          Practice once a day. Streak counts only when your submission scores ≥ 60.
        </CardDescription>
      </CardHeader>
      <CardContent />
      <CardFooter>
        <Link href={`/problems/${problem.id}`}>
          <Button>Start today&apos;s challenge</Button>
        </Link>
      </CardFooter>
    </Card>
  );
}
