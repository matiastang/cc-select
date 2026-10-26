import React from "react";
import { cn } from "../../lib/cn";

type CardProps = React.HTMLAttributes<HTMLDivElement> & {
  flat?: boolean;
};

export function Card({ children, className, flat = false, ...rest }: CardProps) {
  return (
    <div
      className={cn(
        "ui-card",
        flat && "ui-card--flat",
        "rounded-lg border border-border bg-card p-4 md:p-5",
        !flat && "shadow-sm",
        className,
      )}
      {...rest}
    >
      {children}
    </div>
  );
}
