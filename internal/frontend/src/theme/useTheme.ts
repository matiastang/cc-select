import { useEffect, useState } from "react";
import { Theme, applyTheme, getStoredTheme, setStoredTheme } from "./theme";

export type { Theme } from "./theme";

export function useTheme(): [Theme, (theme: Theme) => void] {
  const [theme, setThemeState] = useState<Theme>(() => getStoredTheme());

  useEffect(() => {
    applyTheme(theme);
  }, [theme]);

  const setTheme = (next: Theme) => {
    setThemeState(next);
    setStoredTheme(next);
  };

  return [theme, setTheme];
}
