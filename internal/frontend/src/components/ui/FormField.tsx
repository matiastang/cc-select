import React, { useId } from "react";

type FormFieldProps = {
  label: React.ReactNode;
  htmlFor?: string;
  helper?: React.ReactNode;
  children: React.ReactNode;
  className?: string;
};

export function FormField({ label, htmlFor, helper, children, className = "" }: FormFieldProps) {
  const helperId = useId();
  const child = React.isValidElement(children)
    ? React.cloneElement(children, {
        "aria-describedby": helper ? helperId : undefined,
      } as Record<string, unknown>)
    : children;

  return (
    <div className={`ui-form-field ${className}`.trim()}>
      <label className="ui-label" htmlFor={htmlFor}>
        {label}
      </label>
      {child}
      {helper && (
        <span id={helperId} className="ui-form-field__helper">
          {helper}
        </span>
      )}
    </div>
  );
}
