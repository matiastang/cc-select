import React from "react";
import {
  Check,
  ChevronDown,
  ChevronRight,
  Eye,
  EyeOff,
  Globe,
  KeyRound,
  Monitor,
  Moon,
  Pencil,
  Sun,
  Trash2,
  type LucideProps,
} from "lucide-react";
import { cn } from "../../lib/cn";

// 兼容转发层：旧 IconName 映射到 lucide 同义图标；新代码直接 import lucide。
// （contracts/frontend §3——Icon 保留为兼容层，逐步退役。）
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
} & Omit<LucideProps, "size" | "className">;

const ICONS: Record<IconName, React.ComponentType<LucideProps>> = {
  chevronDown: ChevronDown,
  chevronRight: ChevronRight,
  eye: Eye,
  eyeOff: EyeOff,
  check: Check,
  globe: Globe,
  key: KeyRound,
  trash: Trash2,
  pencil: Pencil,
  sun: Sun,
  moon: Moon,
  monitor: Monitor,
};

export function Icon({ name, size = 16, className, ...rest }: IconProps) {
  const Cmp = ICONS[name];
  return <Cmp aria-hidden size={size} className={cn(className)} {...rest} />;
}
