import React from "react";

type CardProps = {
  children: React.ReactNode;
  className?: string;
  flat?: boolean;
};

export function Card({ children, className = "", flat = false }: CardProps) {
  const classes = ["ui-card", flat ? "ui-card--flat" : "", className].filter(Boolean).join(" ");
  return <div className={classes}>{children}</div>;
}
