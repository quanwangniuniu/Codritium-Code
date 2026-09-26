import * as React from "react";
import { cn } from "@/lib/utils";

export const Textarea = React.forwardRef<HTMLTextAreaElement, React.TextareaHTMLAttributes<HTMLTextAreaElement>>(
  function Textarea({ className, ...props }, ref) {
    return (
      <textarea
        ref={ref}
        className={cn(
          "block w-full rounded-md border border-divider bg-canvas text-ink",
          "px-3 py-2 text-sm font-mono leading-6",
          "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/50",
          "placeholder:text-faint",
          className,
        )}
        {...props}
      />
    );
  },
);
