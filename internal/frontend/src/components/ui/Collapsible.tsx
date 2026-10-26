import React, { useId } from "react";
import { ChevronRight } from "lucide-react";
import { cn } from "../../lib/cn";

type CollapsibleProps = {
  title: React.ReactNode;
  open: boolean;
  onToggle: () => void;
  children: React.ReactNode;
  "data-testid"?: string;
};

export function Collapsible({
  title,
  open,
  onToggle,
  children,
  "data-testid": testId,
}: CollapsibleProps) {
  const generatedId = useId();
  const contentId = `${generatedId}-content`;

  return (
    <div className="ui-collapsible rounded-md border border-border">
      <button
        type="button"
        className="ui-collapsible__header flex w-full items-center gap-1.5 px-3 py-2 text-sm font-medium text-text hover:bg-bg"
        onClick={onToggle}
        aria-expanded={open}
        aria-controls={contentId}
        data-testid={testId}
      >
        <ChevronRight
          size={16}
          className={cn("text-muted transition-transform", open && "rotate-90")}
        />
        <span>{title}</span>
      </button>
      <div
        id={contentId}
        className="ui-collapsible__content border-t border-border px-3 py-3"
        hidden={!open}
      >
        {children}
      </div>
    </div>
  );
}
