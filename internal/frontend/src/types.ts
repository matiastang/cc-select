import { APIFormat, AuthField, Preset, PresetDetail } from "./presets/presets";

export type Provider = {
  id: string;
  name: string;
  env: Record<string, string>;
  hasKey: boolean;
  varKeys: string[];
  isolationMode: string;
};

// ProviderDetail corresponds to backend GET /providers/{id}: includes raw settings.json from disk.
export type ProviderDetail = {
  id: string;
  name: string;
  settings: unknown;
  isolationMode: string;
  preset?: string;
  apiFormat?: APIFormat;
  authField?: AuthField;
};

export type IsolationMode = "" | "settings-only" | "full";

export type { Preset, PresetDetail };
