import React from "react";

export type IconName =
  | "chevronDown"
  | "chevronRight"
  | "eye"
  | "eyeOff"
  | "check"
  | "globe"
  | "key"
  | "trash"
  | "pencil"
  | "sun"
  | "moon"
  | "monitor";

type IconProps = {
  name: IconName;
  size?: number;
  className?: string;
};

const ICONS: Record<IconName, React.ReactNode> = {
  chevronDown: (
    <path d="m6 9 6 6 6-6" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
  ),
  chevronRight: (
    <path d="m9 18 6-6-6-6" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
  ),
  eye: (
    <>
      <path d="M2 12s3-7 10-7 10 7 10 7-3 7-10 7-10-7-10-7Z" fill="none" stroke="currentColor" strokeWidth="2" />
      <circle cx="12" cy="12" r="3" fill="none" stroke="currentColor" strokeWidth="2" />
    </>
  ),
  eyeOff: (
    <>
      <path d="M9.88 9.88a3 3 0 1 0 4.24 4.24" fill="none" stroke="currentColor" strokeWidth="2" />
      <path d="M10.73 5.08A10.43 10.43 0 0 1 12 5c7 0 10 7 10 7a13.16 13.16 0 0 1-1.67 2.68" fill="none" stroke="currentColor" strokeWidth="2" />
      <path d="M6.61 6.61A13.526 13.526 0 0 0 2 12s3 7 10 7a9.74 9.74 0 0 0 5.39-1.61" fill="none" stroke="currentColor" strokeWidth="2" />
      <line x1="2" x2="22" y1="2" y2="22" stroke="currentColor" strokeWidth="2" />
    </>
  ),
  check: (
    <path d="M20 6 9 17l-5-5" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
  ),
  globe: (
    <>
      <circle cx="12" cy="12" r="10" fill="none" stroke="currentColor" strokeWidth="2" />
      <path d="M2 12h20" fill="none" stroke="currentColor" strokeWidth="2" />
      <path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z" fill="none" stroke="currentColor" strokeWidth="2" />
    </>
  ),
  key: (
    <>
      <circle cx="7.5" cy="15.5" r="5.5" fill="none" stroke="currentColor" strokeWidth="2" />
      <path d="m21 2-9.6 9.6" fill="none" stroke="currentColor" strokeWidth="2" />
      <path d="m15.5 7.5 3 3L22 7l-3-3-3.5 3.5Z" fill="none" stroke="currentColor" strokeWidth="2" />
    </>
  ),
  trash: (
    <>
      <path d="M3 6h18" fill="none" stroke="currentColor" strokeWidth="2" />
      <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2" fill="none" stroke="currentColor" strokeWidth="2" />
      <line x1="10" x2="10" y1="11" y2="17" stroke="currentColor" strokeWidth="2" />
      <line x1="14" x2="14" y1="11" y2="17" stroke="currentColor" strokeWidth="2" />
    </>
  ),
  pencil: (
    <>
      <path d="M17 3a2.85 2.83 0 1 1 4 4L7.5 20.5 2 22l1.5-5.5Z" fill="none" stroke="currentColor" strokeWidth="2" />
    </>
  ),
  sun: (
    <>
      <circle cx="12" cy="12" r="4" fill="none" stroke="currentColor" strokeWidth="2" />
      <path d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M6.34 17.66l-1.41 1.41M19.07 4.93l-1.41 1.41" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
    </>
  ),
  moon: (
    <>
      <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" fill="none" stroke="currentColor" strokeWidth="2" />
    </>
  ),
  monitor: (
    <>
      <rect x="2" y="3" width="20" height="14" rx="2" ry="2" fill="none" stroke="currentColor" strokeWidth="2" />
      <path d="M8 21h8M12 17v4" fill="none" stroke="currentColor" strokeWidth="2" />
    </>
  ),
};

export function Icon({ name, size = 16, className = "" }: IconProps) {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      className={`ui-icon ui-icon--${name} ${className}`.trim()}
      aria-hidden="true"
    >
      {ICONS[name]}
    </svg>
  );
}
