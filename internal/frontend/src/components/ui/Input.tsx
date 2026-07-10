import React from "react";

type InputProps = React.InputHTMLAttributes<HTMLInputElement>;

export function Input({ className = "", ...rest }: InputProps) {
  return <input className={`ui-input ${className}`.trim()} {...rest} />;
}
