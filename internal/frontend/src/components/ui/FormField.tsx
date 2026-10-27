import React, { useId } from "react";
import { cn } from "../../lib/cn";
import { Label } from "./Label";

type FormFieldProps = {
  label: React.ReactNode;
  htmlFor?: string;
  helper?: React.ReactNode;
  children: React.ReactNode;
  className?: string;
};

export function FormField({ label, htmlFor, helper, children, className }: FormFieldProps) {
  const helperId = useId();
  const child = React.isValidElement(children)
    ? React.cloneElement(children, {
        "aria-describedby": helper ? helperId : undefined,
      } as Record<string, unknown>)
    : children;

  return (
    <div className={cn("ui-form-field", "flex flex-col gap-1.5", className)}>
      <Label htmlFor={htmlFor}>{label}</Label>
      {child}
      {helper && (
        <span id={helperId} className="ui-form-field__helper text-xs text-muted">
          {helper}
        </span>
      )}
    </div>
  );
}
