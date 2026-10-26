import React from "react";
import { cn } from "../../lib/cn";

type LabelProps = React.LabelHTMLAttributes<HTMLLabelElement>;

export function Label({ className, ...rest }: LabelProps) {
  return (
    <label
      className={cn(
        "ui-label",
        "block text-sm font-medium text-text",
        "peer-disabled:cursor-not-allowed peer-disabled:opacity-70",
        className,
      )}
      {...rest}
    />
  );
}
