import { Loader2 } from "lucide-react";
import { cn } from "../../lib/cn";

type SpinnerProps = {
  label: string;
  size?: number;
  className?: string;
};

export function Spinner({ label, size = 16, className }: SpinnerProps) {
  return (
    <Loader2
      role="progressbar"
      aria-label={label}
      size={size}
      className={cn("ui-spinner animate-spin text-muted", className)}
    />
  );
}
