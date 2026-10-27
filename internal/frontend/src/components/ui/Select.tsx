import React from "react";
import { ChevronDown } from "lucide-react";
import { cn } from "../../lib/cn";

type SelectProps = React.SelectHTMLAttributes<HTMLSelectElement>;

// 原生 select + 自绘箭头（保留原生无障碍语义与键盘行为）。
export function Select({ className, children, ...rest }: SelectProps) {
  return (
    <div className={cn("ui-select", "relative", className)}>
      <select
        className={cn(
          "ui-select__native",
          "w-full appearance-none rounded-md border border-border bg-input px-3 py-1.5 pr-8 text-sm text-text",
          "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/50",
          "disabled:cursor-not-allowed disabled:opacity-50",
        )}
        {...rest}
      >
        {children}
      </select>
      <ChevronDown
        size={14}
        className="pointer-events-none absolute right-2.5 top-1/2 -translate-y-1/2 text-muted"
      />
    </div>
  );
}
