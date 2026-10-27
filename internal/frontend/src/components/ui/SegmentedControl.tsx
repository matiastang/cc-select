import React, { useCallback, useRef } from "react";
import { cn } from "../../lib/cn";

type SegmentOption<T extends string> = {
  value: T;
  label: React.ReactNode;
};

type SegmentedControlProps<T extends string> = {
  options: SegmentOption<T>[];
  value: T;
  onChange: (value: T) => void;
  id?: string;
  "aria-label"?: string;
  "aria-labelledby"?: string;
  "data-testid"?: string;
};

export function SegmentedControl<T extends string>({
  options,
  value,
  onChange,
  id,
  "aria-label": ariaLabel,
  "aria-labelledby": ariaLabelledBy,
  "data-testid": testId,
}: SegmentedControlProps<T>) {
  const activeIndex = options.findIndex((o) => o.value === value);
  const buttonRefs = useRef<Array<HTMLButtonElement | null>>([]);

  const focusAndSelect = useCallback(
    (nextIndex: number) => {
      const idx = Math.max(0, Math.min(options.length - 1, nextIndex));
      const btn = buttonRefs.current[idx];
      onChange(options[idx].value);
      btn?.focus();
    },
    [onChange, options],
  );

  const handleKeyDown = (e: React.KeyboardEvent<HTMLButtonElement>) => {
    switch (e.key) {
      case "ArrowLeft":
      case "ArrowUp":
        e.preventDefault();
        focusAndSelect(activeIndex - 1);
        break;
      case "ArrowRight":
      case "ArrowDown":
        e.preventDefault();
        focusAndSelect(activeIndex + 1);
        break;
      case "Home":
        e.preventDefault();
        focusAndSelect(0);
        break;
      case "End":
        e.preventDefault();
        focusAndSelect(options.length - 1);
        break;
    }
  };

  return (
    <div
      id={id}
      className="ui-segmented-control inline-flex overflow-hidden rounded-md border border-border bg-card"
      role="radiogroup"
      aria-label={ariaLabel}
      aria-labelledby={ariaLabelledBy}
      data-testid={testId}
    >
      {options.map((option, index) => {
        const active = option.value === value;
        return (
          <button
            key={option.value}
            ref={(el) => {
              buttonRefs.current[index] = el;
            }}
            type="button"
            role="radio"
            aria-checked={active}
            tabIndex={active ? 0 : -1}
            data-value={option.value}
            className={cn(
              "ui-segmented-control__option",
              active && "ui-segmented-control__option--active",
              "px-3 py-1.5 text-sm transition-colors",
              "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/50 focus-visible:-outline-offset-2",
              active
                ? "bg-accent text-accent-contrast"
                : "bg-card text-muted hover:bg-bg hover:text-text",
            )}
            onClick={() => onChange(option.value)}
            onKeyDown={handleKeyDown}
          >
            {option.label}
          </button>
        );
      })}
    </div>
  );
}
