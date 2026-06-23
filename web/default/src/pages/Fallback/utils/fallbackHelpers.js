// ============================================================
// fallbackHelpers.js — Fallback 页面纯函数集合
// ============================================================

// ---- 常量 ----

export const PANEL_ITEMS = [
  {
    key: 'gateway',
    title: '模型编辑器',
    description: '管理高质量模型、低成本模型和普通模型部署。',
    icon: 'edit',
    accent: '#0ea5e9',
  },
  {
    key: 'free-pool',
    title: '免费模型池',
    description: '管理免费模型、免费供应商、限额覆盖和自动生成的免费部署。',
    icon: 'cloud',
    accent: '#16a34a',
  },
  {
    key: 'status',
    title: '模型状态',
    description: '显示额度、Token、并发、冷却和恢复按钮，适合日常值守。',
    icon: 'server',
    accent: '#2563eb',
  },
  {
    key: 'metrics',
    title: '使用统计',
    description: '展示请求量、切换次数、成功失败和 token 消耗等原始监控。',
    icon: 'heartbeat',
    accent: '#0f9f9a',
  },
  {
    key: 'scores',
    title: '模型权重',
    description: '用当前分数和趋势图判断哪个模型变好、变差或正在恢复。',
    icon: 'sort numeric down',
    accent: '#7c3aed',
  },
  {
    key: 'alerts',
    title: '异常事件',
    description: '按时间记录限额、手动冷却、自动恢复和全部失败事件。',
    icon: 'bell outline',
    accent: '#d97706',
  },
  {
    key: 'logs',
    title: '切换快照',
    description: '记录切换原因、状态码、耗时和 request id，方便追单次请求。',
    icon: 'exchange',
    accent: '#475569',
  },
];

export const GUIDE_SECTIONS = [
  {
    title: 'CCT API 基于 One API 新增了什么',
    icon: 'rocket',
    items: [
      '虚拟模型：一个对外模型名可以挂多个真实上游模型。',
      '自动切换：当前模型额度耗尽、冷却、报错或并发满时，会尝试下一个可用模型。',
      '权重/顺序模式：健康模型可按权重分流，也可以严格按你排列的顺序使用。',
      '额度与并发控制：每个真实模型都有每日 Token 限额、软硬阈值和并发上限。',
      '运行健康判断：运行数据里直接展示最近成功率、失败率、冷却、额度耗尽和 Top 失败模型/渠道。',
    ],
  },
  {
    title: '第一次应该怎么配置',
    icon: 'settings',
    items: [
      '进入"部署状态"面板，上方"虚拟模型"区域就是配置入口。',
      '新增虚拟模型后，再给它添加真实模型，填写接口地址、密钥和真实模型名。',
      '在虚拟模型里选择"按权重"或"按顺序"；只有按顺序时才开放"编辑顺序"。',
      '设置每日 Token 限额、软/硬限额比例、并发上限，然后点"保存"。保存前会备份旧的 fallback.json。',
    ],
  },
  {
    title: '运行后在哪里看结果',
    icon: 'map signs',
    items: [
      '"模型状态"看每个真实模型是否可用，并可手动冷却或恢复并重置当前周期额度。',
      '"模型权重"看智能排序分数和趋势图，判断谁在变好或变差。',
      '"异常事件"看谁什么时候超额、冷却或恢复；"切换快照"看每次从谁切到谁。',
      '状态页可按配置顺序、Token 用量或模型名称排序，点击模块只刷新下方内容。',
    ],
  },
];

export const PANEL_KEYS = new Set(PANEL_ITEMS.map((item) => item.key));

export const STATUS_SORT_OPTIONS = [
  { key: 'config', value: 'config', text: '按配置顺序' },
  { key: 'tokens', value: 'tokens', text: '按 Token 用量' },
  { key: 'model', value: 'model', text: '按模型名称' },
];

export const PANEL_REFRESH_INTERVALS = {
  status: 15000,
  metrics: 30000,
  scores: 15000,
  alerts: 60000,
  logs: 60000,
};

export const METRIC_SAMPLE_STORAGE_KEY = 'fallback_runtime_metric_samples';
export const METRIC_SAMPLE_RETENTION_MS = 65 * 60 * 1000;
export const FIVE_MINUTES_MS = 5 * 60 * 1000;
export const ONE_HOUR_MS = 60 * 60 * 1000;
export const SUCCESS_RATE_WARNING_THRESHOLD = 95;
export const SUCCESS_RATE_CRITICAL_THRESHOLD = 90;
export const FAILURE_RATE_WARNING_THRESHOLD = 5;
export const FAILURE_RATE_CRITICAL_THRESHOLD = 15;
export const SCORE_CHART_VISIBLE_LIMIT = 8;

export const emptyConfigMeta = {
  deploymentMap: {},
  orderMap: {},
  virtualMap: {},
  virtualOrder: [],
};

// ---- 格式化函数 ----

export const formatNumber = (value) => {
  const number = Number(value);
  if (!Number.isFinite(number)) {
    return '-';
  }
  return new Intl.NumberFormat('zh-CN').format(number);
};

export const formatPercent = (value) => {
  if (value === undefined || value === null || value === '') {
    return '-';
  }
  if (typeof value === 'string') {
    return value;
  }
  const number = Number(value);
  if (!Number.isFinite(number)) {
    return String(value);
  }
  return `${number.toFixed(1)}%`;
};

export const formatConcurrency = (row) => {
  const inFlight = Number(row?.in_flight_requests || 0);
  const limit = Number(row?.max_concurrent_requests || 0);
  if (limit > 0) {
    return `${formatNumber(inFlight)} / ${formatNumber(limit)}`;
  }
  return `${formatNumber(inFlight)} / 不限`;
};

export const formatTime = (value) => {
  if (!value) {
    return '-';
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return String(value);
  }
  return date.toLocaleString('zh-CN', { hour12: false });
};

export const formatInterval = (milliseconds) => {
  if (milliseconds >= 60000) {
    return `${Math.round(milliseconds / 60000)} 分钟`;
  }
  return `${Math.round(milliseconds / 1000)} 秒`;
};

export const formatChartTime = (value) => {
  if (!value) {
    return '-';
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return String(value);
  }
  return date.toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  });
};

export const formatDuration = (milliseconds) => {
  const number = Number(milliseconds);
  if (!Number.isFinite(number) || number <= 0) {
    return '不足 1 分钟';
  }
  const minutes = Math.max(1, Math.round(number / 60000));
  if (minutes >= 60) {
    const hours = Math.floor(minutes / 60);
    const restMinutes = minutes % 60;
    return restMinutes > 0 ? `${hours} 小时 ${restMinutes} 分钟` : `${hours} 小时`;
  }
  return `${minutes} 分钟`;
};

// ---- 业务逻辑函数 ----

export const getPanelKey = (panel) => {
  if (panel === 'dashboard') {
    return 'gateway';
  }
  if (panel === 'legacy') {
    return 'gateway';
  }
  return PANEL_KEYS.has(panel) ? panel : 'gateway';
};

export const getScoreDeltaMeta = (rows, deploymentId) => {
  if (!Array.isArray(rows)) {
    return null;
  }
  const points = rows
    .map((row) => {
      const value = Number(row?.values?.[deploymentId]);
      return Number.isFinite(value) ? value : null;
    })
    .filter((value) => value !== null);

  if (points.length < 2) {
    return null;
  }

  const current = points[points.length - 1];
  const previous = points[points.length - 2];
  const delta = Number((current - previous).toFixed(2));

  if (delta > 0) {
    return { direction: 'up', icon: 'arrow up', text: `+${delta.toFixed(2)}` };
  }
  if (delta < 0) {
    return { direction: 'down', icon: 'arrow down', text: delta.toFixed(2) };
  }
  return { direction: 'flat', icon: 'minus', text: '0.00' };
};

export const translateFallbackReason = (reason) => {
  const text = String(reason || '').trim();
  if (!text) {
    return '-';
  }

  const lower = text.toLowerCase();
  const translations = [
    {
      match: 'deployment reached soft daily token limit',
      text: '已达到每日软额度上限，已自动切换',
    },
    {
      match: 'set inference limit',
      text: '已达到推理额度上限，模型已暂停，已自动切换',
    },
    {
      match: 'model service has been paused',
      text: '模型服务已暂停，已自动切换',
    },
    {
      match: 'safe experience mode',
      text: '已触发安全体验模式限制，已自动切换',
    },
    {
      match: 'rate limit',
      text: '触发限流，已自动切换',
    },
    {
      match: 'too many requests',
      text: '请求过多，已自动切换',
    },
  ];

  for (const item of translations) {
    if (lower.includes(item.match)) {
      return item.text;
    }
  }

  return text;
};

export const getScoreSeriesPoints = (rows, deploymentId, currentScore) => {
  const points = (Array.isArray(rows) ? rows : [])
    .map((row) => Number(row?.values?.[deploymentId]))
    .filter((value) => Number.isFinite(value));
  const current = Number(currentScore);

  if (Number.isFinite(current)) {
    const last = points[points.length - 1];
    if (!Number.isFinite(last) || Math.abs(last - current) > 0.001) {
      points.push(current);
    }
  }

  return points.slice(-10);
};

export const getScoreBand = (value) => {
  const score = Number(value);
  if (!Number.isFinite(score)) {
    return 'mid';
  }
  if (score >= 90) {
    return 'high';
  }
  if (score >= 70) {
    return 'mid';
  }
  if (score >= 50) {
    return 'low';
  }
  return 'critical';
};

export const normalizeMetricSamples = (samples) =>
  (Array.isArray(samples) ? samples : [])
    .map((sample) => ({
      timestamp: Number(sample.timestamp || sample.ts || 0),
      requests: Number(sample.requests || 0),
      switches: Number(sample.switches || 0),
      success: Number(sample.success || 0),
      failed: Number(sample.failed || 0),
    }))
    .filter(
      (sample) =>
        Number.isFinite(sample.timestamp) &&
        sample.timestamp > 0 &&
        Number.isFinite(sample.success) &&
        Number.isFinite(sample.failed)
    )
    .sort((left, right) => left.timestamp - right.timestamp);

export const loadMetricSamples = () => {
  if (typeof window === 'undefined') {
    return [];
  }
  try {
    const cutoff = Date.now() - METRIC_SAMPLE_RETENTION_MS;
    return normalizeMetricSamples(
      JSON.parse(window.localStorage.getItem(METRIC_SAMPLE_STORAGE_KEY) || '[]')
    ).filter((sample) => sample.timestamp >= cutoff);
  } catch (e) {
    return [];
  }
};

export const saveMetricSamples = (samples) => {
  if (typeof window === 'undefined') {
    return;
  }
  try {
    window.localStorage.setItem(
      METRIC_SAMPLE_STORAGE_KEY,
      JSON.stringify(samples)
    );
  } catch (e) {
    // Ignore storage failures; the panel can still use in-memory samples.
  }
};

export const calculateWindowRate = (samples, windowMs) => {
  const sortedSamples = normalizeMetricSamples(samples);
  if (sortedSamples.length < 2) {
    return {
      successRate: null,
      failureRate: null,
      handled: 0,
      success: 0,
      failed: 0,
      spanMs: 0,
      state: 'sampling',
    };
  }

  const latest = sortedSamples[sortedSamples.length - 1];
  const cutoff = latest.timestamp - windowMs;
  const firstInsideIndex = sortedSamples.findIndex(
    (sample) => sample.timestamp >= cutoff
  );
  const baselineIndex =
    firstInsideIndex > 0 ? firstInsideIndex - 1 : Math.max(0, firstInsideIndex);
  const baseline = sortedSamples[baselineIndex];

  if (!baseline || baseline.timestamp === latest.timestamp) {
    return {
      successRate: null,
      failureRate: null,
      handled: 0,
      success: 0,
      failed: 0,
      spanMs: 0,
      state: 'sampling',
    };
  }

  const success = latest.success - baseline.success;
  const failed = latest.failed - baseline.failed;
  const requests = latest.requests - baseline.requests;
  const switches = latest.switches - baseline.switches;

  if (success < 0 || failed < 0 || requests < 0 || switches < 0) {
    return {
      successRate: null,
      failureRate: null,
      handled: 0,
      success: 0,
      failed: 0,
      spanMs: latest.timestamp - baseline.timestamp,
      state: 'reset',
    };
  }

  const handled = success + failed;
  return {
    successRate: handled > 0 ? (success / handled) * 100 : null,
    failureRate: handled > 0 ? (failed / handled) * 100 : null,
    handled,
    success,
    failed,
    requests,
    switches,
    spanMs: latest.timestamp - baseline.timestamp,
    state: handled > 0 ? 'ready' : 'idle',
  };
};

export const getWindowRateNote = (rate) => {
  if (rate.state === 'reset') {
    return '计数器重置后重新采样';
  }
  if (rate.state === 'sampling') {
    return '采样中';
  }
  if (rate.state === 'idle') {
    return `近 ${formatDuration(rate.spanMs)} 无完成请求`;
  }
  return `${formatNumber(rate.handled)} 次完成请求，采样 ${formatDuration(
    rate.spanMs
  )}`;
};

export const getSuccessRateLevel = (rate) => {
  if (rate.successRate === null) {
    return '';
  }
  if (rate.successRate < SUCCESS_RATE_CRITICAL_THRESHOLD) {
    return 'critical';
  }
  if (rate.successRate < SUCCESS_RATE_WARNING_THRESHOLD) {
    return 'warning';
  }
  return 'normal';
};

export const getFailureRateLevel = (rate) => {
  if (rate.failureRate === null) {
    return '';
  }
  if (rate.failureRate >= FAILURE_RATE_CRITICAL_THRESHOLD) {
    return 'critical';
  }
  if (rate.failureRate >= FAILURE_RATE_WARNING_THRESHOLD) {
    return 'warning';
  }
  return 'normal';
};

export const isFutureTime = (value, now = Date.now()) => {
  if (!value) {
    return false;
  }
  const time = new Date(value).getTime();
  return Number.isFinite(time) && time > now;
};

export const isRecentTime = (value, windowMs, now = Date.now()) => {
  if (!value) {
    return false;
  }
  const time = new Date(value).getTime();
  return Number.isFinite(time) && now - time <= windowMs;
};

export const isQuotaExhaustedRow = (row) => {
  const alertType = row?.alert_type;
  if (alertType === 'exhausted' || alertType === 'hard_limit') {
    return true;
  }
  const usedTokens = Number(row?.used_tokens || 0);
  const dailyLimit = Number(row?.daily_limit || 0);
  return dailyLimit > 0 && usedTokens >= dailyLimit;
};

export const buildDeploymentMeta = (config) => {
  if (!config) {
    return emptyConfigMeta;
  }

  const deploymentMap = {};
  const orderMap = {};
  const virtualMap = {};
  const virtualOrder = [];
  let orderIndex = 0;

  (config.virtual_models || []).forEach((vm) => {
    if (vm?.name) {
      virtualOrder.push(vm.name);
    }
    (vm?.fallback_order || []).forEach((deploymentId) => {
      if (orderMap[deploymentId] === undefined) {
        orderMap[deploymentId] = orderIndex;
        orderIndex += 1;
      }
      if (!virtualMap[deploymentId]) {
        virtualMap[deploymentId] = [];
      }
      if (vm?.name && !virtualMap[deploymentId].includes(vm.name)) {
        virtualMap[deploymentId].push(vm.name);
      }
    });
  });

  (config.deployments || []).forEach((dep, index) => {
    if (!dep?.id) {
      return;
    }
    deploymentMap[dep.id] = dep;
    if (orderMap[dep.id] === undefined) {
      orderMap[dep.id] = orderIndex + index;
    }
  });

  return {
    deploymentMap,
    orderMap,
    virtualMap,
    virtualOrder,
  };
};

export const getStatusMeta = (row) => {
  switch (row.alert_type) {
    case 'cooldown':
      return {
        text: '冷却中',
        color: 'orange',
        note: `冷却至 ${formatTime(row.cooldown_until)}`,
      };
    case 'exhausted':
      return {
        text: '已耗尽',
        color: 'red',
        note: `耗尽至 ${formatTime(row.exhausted_until)}`,
      };
    case 'hard_limit':
      return {
        text: '硬限额',
        color: 'red',
        note: `用量 ${formatPercent(row.usage_percent)}`,
      };
    case 'soft_limit':
      return {
        text: '软限额',
        color: 'yellow',
        note: `用量 ${formatPercent(row.usage_percent)}`,
      };
    default:
      return {
        text: '可用',
        color: row.enabled === false ? 'grey' : 'green',
        note: row.enabled === false ? '已禁用' : '可用',
      };
  }
};

export const getLevelColor = (level) => {
  switch (level) {
    case 'critical':
      return 'red';
    case 'warning':
      return 'yellow';
    case 'info':
      return 'blue';
    default:
      return 'green';
  }
};

export const parseMetricLabels = (labelText) => {
  const labels = {};
  String(labelText || '')
    .split(',')
    .map((part) => part.trim())
    .filter(Boolean)
    .forEach((part) => {
      const match = part.match(/^([^=]+)="?(.*?)"?$/);
      if (match) {
        labels[match[1]] = match[2];
      }
    });
  return labels;
};

export const parseMetrics = (metricsText) =>
  String(metricsText || '')
    .split('\n')
    .map((line) => line.trim())
    .filter((line) => line && !line.startsWith('#'))
    .map((line, index) => {
      const match = line.match(/^([a-zA-Z_:][a-zA-Z0-9_:]*)(?:\{([^}]*)\})?\s+(.+)$/);
      if (!match) {
        const parts = line.split(/\s+/);
        return {
          key: `${index}:${parts[0]}`,
          name: parts[0],
          displayName: parts[0],
          labels: {},
          value: parts.slice(1).join(' '),
          numericValue: Number(parts.slice(1).join(' ')),
        };
      }
      const [, name, labelText, value] = match;
      return {
        key: `${index}:${name}:${labelText || ''}`,
        name,
        displayName: labelText ? `${name}{${labelText}}` : name,
        labels: parseMetricLabels(labelText),
        value,
        numericValue: Number(value),
      };
    });
