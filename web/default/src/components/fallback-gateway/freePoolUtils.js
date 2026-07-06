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

const applyLimitOverride = (base, overrides, field) => {
  if (!overrides || overrides[field] === undefined || overrides[field] === null || overrides[field] === '') {
    return base;
  }
  const parsed = Number(overrides[field]);
  return Number.isFinite(parsed) ? parsed : base;
};

export const buildFreeProviderRows = (freeProviders = {}, catalog = []) => {
  const providers = freeProviders && typeof freeProviders === 'object' ? freeProviders : {};
  const entries = Array.isArray(catalog) ? catalog : [];
  const rows = [];
  const seen = new Set();

  const buildRow = (name, saved = {}, meta = {}, configured = false) => {
    const limitsOverride = { ...(saved.limits_override || {}) };
    const rpmLimit = pickNumber(meta.rpm_limit, saved.default_rpm);
    const rpdLimit = pickNumber(meta.rpd_limit, saved.default_rpd);
    const tpmLimit = pickNumber(meta.tpm_limit, saved.default_tpm);
    const tpdLimit = pickNumber(meta.tpd_limit, saved.default_tpd);

    return {
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
      rpm_limit: applyLimitOverride(rpmLimit, limitsOverride, 'rpm_limit'),
      rpd_limit: applyLimitOverride(rpdLimit, limitsOverride, 'rpd_limit'),
      tpm_limit: applyLimitOverride(tpmLimit, limitsOverride, 'tpm_limit'),
      tpd_limit: applyLimitOverride(tpdLimit, limitsOverride, 'tpd_limit'),
      limits_override: limitsOverride,
      quirks: saved.quirks || meta.quirks || null,
      model_fetch_mode: saved.model_fetch_mode || meta.model_fetch_mode || '',
      provider_id: saved.provider_id || meta.provider_id || name,
      default_base_url: saved.default_base_url || meta.default_base_url || '',
    };
  };

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

export const indexUsageRows = (rows = []) => {
  const index = {};
  (Array.isArray(rows) ? rows : []).forEach((row) => {
    const provider = String(row.provider || '').trim().toLowerCase();
    const keyHash = String(row.key_hash || '').trim();
    if (!provider || !keyHash) return;
    const key = `${provider}:${keyHash}`;
    if (!index[key]) index[key] = [];
    index[key].push(row);
  });
  return index;
};

export const buildClearKeysProviderConfig = (freeProviders = {}, providerKey) => {
  const current = freeProviders && typeof freeProviders === 'object' ? freeProviders : {};
  const existing = current[providerKey] || {};
  const nextProvider = { ...existing, clear_keys: true };
  delete nextProvider.keys;
  return {
    ...current,
    [providerKey]: nextProvider,
  };
};

export const buildReplaceKeysProviderConfig = (freeProviders = {}, providerKey, value) => {
  const current = freeProviders && typeof freeProviders === 'object' ? freeProviders : {};
  const existing = current[providerKey] || {};
  const keys = String(value || '')
    .split(/\r?\n/)
    .map((key) => key.trim())
    .filter((key) => key && !key.includes('*'));
  const nextProvider = { ...existing, keys };
  delete nextProvider.clear_keys;
  return {
    ...current,
    [providerKey]: nextProvider,
  };
};

export const loadFreePoolDashboardData = async ({
  getGatewayConfig,
  getRuntimeStatus,
  getFreePoolUsage,
}) => {
  const usageRequest = getFreePoolUsage()
    .then((usageRes) => usageRes)
    .catch(() => null);
  const [configRes, runtimeRes, usageRes] = await Promise.all([
    getGatewayConfig(),
    getRuntimeStatus(),
    usageRequest,
  ]);
  const usageData = usageRes?.data || {};
  return {
    configData: configRes.data || {},
    runtimeData: runtimeRes.data || {},
    usageRows: usageData.success !== false && Array.isArray(usageData.data) ? usageData.data : [],
  };
};
