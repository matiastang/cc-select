import React, { useId } from "react";
import { Icon } from "./Icon";

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
    <div className="ui-collapsible">
      <button
        type="button"
        className="ui-collapsible__header"
        onClick={onToggle}
        aria-expanded={open}
        aria-controls={contentId}
        data-testid={testId}
      >
        <Icon name="chevronRight" size={16} />
        <span>{title}</span>
      </button>
      <div id={contentId} className="ui-collapsible__content" hidden={!open}>
        {children}
      </div>
    </div>
  );
}
