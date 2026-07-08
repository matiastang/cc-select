// Preset types and API helpers mirror the backend /api/v1/presets contract.

export type APIFormat = "anthropic" | "openai_chat" | "openai_responses" | "gemini_native";

export type AuthField = "ANTHROPIC_AUTH_TOKEN" | "ANTHROPIC_API_KEY";

export const AUTH_FIELDS: AuthField[] = ["ANTHROPIC_AUTH_TOKEN", "ANTHROPIC_API_KEY"];

export type PresetCategory =
  | "official"
  | "cn_official"
  | "cloud_provider"
  | "aggregator"
  | "third_party"
  | "custom";

export type Preset = {
  id: string;
  displayName: string;
  category: PresetCategory;
  websiteURL?: string;
  apiKeyURL?: string;
  apiFormat: APIFormat;
  authField?: AuthField;
  requiredVars: string[];
  optionalVars: string[];
  oauth: boolean;
};

export type PresetDetail = Preset & {
  envTemplate: Record<string, string>;
};

export type PresetListResponse = {
  presets: Preset[];
  categories: PresetCategory[];
};

export const PRESETS_API = "/api/v1/presets";

export async function fetchPresets(): Promise<PresetListResponse> {
  const r = await fetch(PRESETS_API);
  if (!r.ok) throw new Error(`failed to fetch presets: ${r.status}`);
  return r.json();
}

export async function fetchPreset(id: string): Promise<PresetDetail> {
  const r = await fetch(`${PRESETS_API}/${id}`);
  if (!r.ok) throw new Error(`failed to fetch preset ${id}: ${r.status}`);
  return r.json();
}

export const CUSTOM_PRESET_ID = "custom";

export function isCustomPreset(id: string): boolean {
  return id === CUSTOM_PRESET_ID || id === "";
}

// Known model-mapping / common Claude env keys surfaced by the structured form.
export const SONNET_MODEL_KEY = "ANTHROPIC_DEFAULT_SONNET_MODEL";
export const OPUS_MODEL_KEY = "ANTHROPIC_DEFAULT_OPUS_MODEL";
export const HAIKU_MODEL_KEY = "ANTHROPIC_DEFAULT_HAIKU_MODEL";
export const FABLE_MODEL_KEY = "ANTHROPIC_DEFAULT_FABLE_MODEL";
export const SUBAGENT_MODEL_KEY = "CLAUDE_CODE_SUBAGENT_MODEL";

export const MODEL_MAPPING_KEYS = [
  SONNET_MODEL_KEY,
  OPUS_MODEL_KEY,
  HAIKU_MODEL_KEY,
  FABLE_MODEL_KEY,
  SUBAGENT_MODEL_KEY,
] as const;

export const COMMON_SETTINGS_KEYS = [
  "CLAUDE_CODE_HIDE_AI_INDICATOR",
  "CLAUDE_CODE_ENABLE_TEAMMATES",
  "CLAUDE_CODE_ENABLE_TOOL_SEARCH",
  "CLAUDE_CODE_MAX_THINKING",
  "CLAUDE_CODE_DISABLE_AUTOUPDATE",
] as const;

export const BASE_URL_KEY = "ANTHROPIC_BASE_URL";
export const MODEL_KEY = "ANTHROPIC_MODEL";

// Apply a preset template and merge user overrides. Missing placeholders are left as-is.
export function applyPresetTemplate(
  preset: PresetDetail,
  overrides: Record<string, string>
): Record<string, string> {
  const out: Record<string, string> = {};
  for (const [k, v] of Object.entries(preset.envTemplate)) {
    out[k] = v;
  }
  for (const [k, v] of Object.entries(overrides)) {
    if (v !== "") out[k] = v;
  }
  return out;
}

// Extract ${PLACEHOLDER} names from a string.
export function placeholdersIn(value: string): string[] {
  const seen = new Set<string>();
  const re = /\$\{([A-Za-z0-9_]+)\}/g;
  let m: RegExpExecArray | null;
  while ((m = re.exec(value)) !== null) {
    seen.add(m[1]);
  }
  return Array.from(seen);
}

// Return required env keys that are still placeholders in the final env.
export function missingRequired(
  preset: PresetDetail,
  env: Record<string, string>
): string[] {
  const missing: string[] = [];
  for (const key of preset.requiredVars) {
    const value = env[key] ?? "";
    if (value === "" || placeholdersIn(value).length > 0) {
      missing.push(key);
    }
  }
  return missing;
}

// Group presets by category in the order returned by the backend.
export function groupPresetsByCategory(
  presets: Preset[],
  categories: PresetCategory[]
): Record<PresetCategory, Preset[]> {
  const groups: Partial<Record<PresetCategory, Preset[]>> = {};
  for (const cat of categories) {
    groups[cat] = [];
  }
  for (const p of presets) {
    groups[p.category] = groups[p.category] ?? [];
    groups[p.category]!.push(p);
  }
  return groups as Record<PresetCategory, Preset[]>;
}
