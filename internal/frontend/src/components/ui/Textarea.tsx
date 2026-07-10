import React from "react";

type TextareaProps = React.TextareaHTMLAttributes<HTMLTextAreaElement>;

export function Textarea({ className = "", ...rest }: TextareaProps) {
  return <textarea className={`ui-textarea ${className}`.trim()} {...rest} />;
}
