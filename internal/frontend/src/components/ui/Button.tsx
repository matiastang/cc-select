import React from "react";
import { cn } from "../../lib/cn";
import { Icon, IconName } from "./Icon";

type ButtonVariant = "primary" | "secondary" | "danger" | "ghost";
type ButtonSize = "sm" | "md";

const VARIANT_CLASSES: Record<ButtonVariant, string> = {
  primary: "bg-accent text-accent-contrast hover:bg-accent-hover border border-transparent",
  secondary: "bg-card text-text hover:bg-bg border border-border",
  danger: "bg-danger text-danger-contrast hover:bg-danger-hover border border-transparent",
  ghost: "bg-transparent text-text hover:bg-bg border border-transparent",
};

const SIZE_CLASSES: Record<ButtonSize, string> = {
  sm: "px-2.5 py-1 text-xs gap-1",
  md: "px-3.5 py-1.5 text-sm gap-1.5",
};

type ButtonProps = React.ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: ButtonVariant;
  size?: ButtonSize;
  icon?: IconName;
};

export function Button({
  children,
  type = "button",
  variant = "primary",
  size = "md",
  icon,
  className,
  ...rest
}: ButtonProps) {
  return (
    <button
      type={type}
      className={cn(
        "ui-button",
        `ui-button--${variant}`,
        size === "sm" && "ui-button--sm",
        "inline-flex items-center justify-center rounded-md font-medium",
        "transition-colors duration-[var(--transition-fast)]",
        "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/50",
        "disabled:pointer-events-none disabled:opacity-50",
        VARIANT_CLASSES[variant],
        SIZE_CLASSES[size],
        className,
      )}
      {...rest}
    >
      {icon && <Icon name={icon} size={size === "sm" ? 14 : 16} />}
      {children}
    </button>
  );
}
