export const AUTO_FREE_DEPLOYMENT_RE = /^free:([a-z0-9][a-z0-9_-]*)-(\d+|[a-f0-9]{8})$/i;

export const isAutoFreeDeploymentId = (id) =>
  AUTO_FREE_DEPLOYMENT_RE.test(String(id || ''));

export const providerFromDeploymentId = (id) => {
  const match = String(id || '').match(AUTO_FREE_DEPLOYMENT_RE);
  return match ? match[1].toLowerCase() : '-';
};

// 注意：这是"自动免费池体系"专用判定——只认 free:openrouter-x / free:groq-x
// 这种 recognized provider 的 id。跟 deploymentMeta.js 的通用 isFreeDeployment
// 是不同概念：后者接受任何 free:* 前缀和 pool==='free'。别混用。
export const isAutoFreeDeployment = (id, dep) =>
  dep?.pool === 'free' || isAutoFreeDeploymentId(id);

const normalizeProviderName = (name) => String(name || '').trim().toLowerCase();

const pickNumber = (...values) => {
  for (const value of values) {
    const parsed = Number(value);
    if (Number.isFinite(parsed)) return parsed;
  }
  return 0;
};

const pickBoolean = (...values) => {
  for (const value of values) {
    if (typeof value === 'boolean') return value;
  }
  return false;
};

const cloneArray = (value) => (Array.isArray(value) ? [...value] : []);

export const buildFreeProviderRows = (freeProviders = {}, catalog = []) => {
  const providers = freeProviders && typeof freeProviders === 'object' ? freeProviders : {};
  const entries = Array.isArray(catalog) ? catalog : [];
  const rows = [];
  const seen = new Set();

  const buildRow = (name, saved = {}, meta = {}, configured = false) => ({
    ...meta,
    ...saved,
    name,
    configured,
    enabled: pickBoolean(saved.enabled, meta.enabled),
    key_count: cloneArray(saved.keys).length > 0
      ? cloneArray(saved.keys).length
      : pickNumber(saved.key_count, meta.key_count),
    models: cloneArray(saved.models).length > 0 ? cloneArray(saved.models) : cloneArray(meta.models),
    default_models: cloneArray(saved.default_models).length > 0
      ? cloneArray(saved.default_models)
      : cloneArray(meta.default_models),
    requires_key: pickBoolean(saved.requires_key, meta.requires_key),
    keyless: pickBoolean(saved.keyless, meta.keyless),
    supports_vision: pickBoolean(saved.supports_vision, meta.supports_vision),
    supports_stream: pickBoolean(saved.supports_stream, meta.supports_stream),
    supports_tools: pickBoolean(saved.supports_tools, meta.supports_tools),
    supports_json: pickBoolean(saved.supports_json, meta.supports_json),
    rpm_limit: pickNumber(meta.rpm_limit, saved.default_rpm),
    rpd_limit: pickNumber(meta.rpd_limit, saved.default_rpd),
    tpm_limit: pickNumber(meta.tpm_limit, saved.default_tpm),
    tpd_limit: pickNumber(meta.tpd_limit, saved.default_tpd),
    limits_override: { ...(saved.limits_override || {}) },
    quirks: saved.quirks || meta.quirks || null,
    model_fetch_mode: saved.model_fetch_mode || meta.model_fetch_mode || '',
    provider_id: saved.provider_id || meta.provider_id || name,
    default_base_url: saved.default_base_url || meta.default_base_url || '',
  });

  entries.forEach((entry) => {
    const name = normalizeProviderName(entry.name);
    if (!name) return;
    const saved = providers[name] || {};
    rows.push(buildRow(name, saved, entry, Object.prototype.hasOwnProperty.call(providers, name)));
    seen.add(name);
  });

  Object.keys(providers)
    .map(normalizeProviderName)
    .filter((name) => name && !seen.has(name))
    .sort()
    .forEach((name) => {
      rows.push(buildRow(name, providers[name], {}, true));
    });

  return rows;
};
