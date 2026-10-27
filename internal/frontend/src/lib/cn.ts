import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

// shadcn/ui 惯例的类合并工具：条件类 + Tailwind 冲突消解。
export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs));
}
