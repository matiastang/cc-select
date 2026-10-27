import React from "react";
import { cn } from "../../lib/cn";
import { Button } from "./Button";

type EmptyStateProps = {
  title: React.ReactNode;
  description?: React.ReactNode;
  actionLabel?: React.ReactNode;
  onAction?: () => void;
  icon?: React.ReactNode;
  className?: string;
  "data-testid"?: string;
};

export function EmptyState({
  title,
  description,
  actionLabel,
  onAction,
  icon,
  className,
  "data-testid": testId,
}: EmptyStateProps) {
  return (
    <div
      data-testid={testId}
      className={cn(
        "ui-empty-state",
        "flex flex-col items-center gap-2 rounded-lg border border-dashed border-border bg-card/50 px-6 py-10 text-center",
        className,
      )}
    >
      {icon && <div className="text-muted">{icon}</div>}
      <div className="text-sm font-medium text-text">{title}</div>
      {description && <div className="max-w-md text-sm text-muted">{description}</div>}
      {actionLabel && onAction && (
        <Button className="mt-2" onClick={onAction}>
          {actionLabel}
        </Button>
      )}
    </div>
  );
}
