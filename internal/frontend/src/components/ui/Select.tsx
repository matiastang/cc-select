import React from "react";

type SelectProps = React.SelectHTMLAttributes<HTMLSelectElement>;

export function Select({ className = "", ...rest }: SelectProps) {
  return (
    <div className={`ui-select ${className}`.trim()}>
      <select className="ui-select__native" {...rest} />
    </div>
  );
}
