import * as React from "react";
import { cn } from "@/shared/lib/cn";

export const Input = React.forwardRef<HTMLInputElement, React.InputHTMLAttributes<HTMLInputElement>>(
  function Input({ className, ...props }, ref) {
    return (
      <input
        ref={ref}
        className={cn(
          "block w-full rounded-md border border-divider bg-canvas text-ink",
          "px-3 h-10 text-sm",
          "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/50",
          "placeholder:text-faint",
          className,
        )}
        {...props}
      />
    );
  },
);
