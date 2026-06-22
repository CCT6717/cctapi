import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import {
  Button,
  Card,
  Dropdown,
  Icon,
  Label,
  Loader,
  Message,
  Popup,
  Table,
} from 'semantic-ui-react';
import {
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts';
import FallbackRuntimePanel from '../../components/FallbackRuntimePanel';
import ModelEditor from '../../components/FallbackConfigPanel';
import FreeModelPool from '../../components/fallback-gateway/FreeModelPool';
import { API, isAdmin, showError, showSuccess } from '../../helpers';
import { clampScore, sortScoreItems } from './scoreUtils';
import './Fallback.css';

const getScoreDeltaMeta = (rows, deploymentId) => {
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

const translateFallbackReason = (reason) => {
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

const PANEL_ITEMS = [
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

const GUIDE_SECTIONS = [
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
      '进入“部署状态”面板，上方“虚拟模型”区域就是配置入口。',
      '新增虚拟模型后，再给它添加真实模型，填写接口地址、密钥和真实模型名。',
      '在虚拟模型里选择“按权重”或“按顺序”；只有按顺序时才开放“编辑顺序”。',
      '设置每日 Token 限额、软/硬限额比例、并发上限，然后点“保存”。保存前会备份旧的 fallback.json。',
    ],
  },
  {
    title: '运行后在哪里看结果',
    icon: 'map signs',
    items: [
      '“模型状态”看每个真实模型是否可用，并可手动冷却或恢复并重置当前周期额度。',
      '“模型权重”看智能排序分数和趋势图，判断谁在变好或变差。',
      '“异常事件”看谁什么时候超额、冷却或恢复；“切换快照”看每次从谁切到谁。',
      '状态页可按配置顺序、Token 用量或模型名称排序，点击模块只刷新下方内容。',
    ],
  },
];

const PANEL_KEYS = new Set(PANEL_ITEMS.map((item) => item.key));

const STATUS_SORT_OPTIONS = [
  { key: 'config', value: 'config', text: '按配置顺序' },
  { key: 'tokens', value: 'tokens', text: '按 Token 用量' },
  { key: 'model', value: 'model', text: '按模型名称' },
];

const PANEL_REFRESH_INTERVALS = {
  status: 15000,
  metrics: 30000,
  scores: 15000,
  alerts: 60000,
  logs: 60000,
};

const METRIC_SAMPLE_STORAGE_KEY = 'fallback_runtime_metric_samples';
const METRIC_SAMPLE_RETENTION_MS = 65 * 60 * 1000;
const FIVE_MINUTES_MS = 5 * 60 * 1000;
const ONE_HOUR_MS = 60 * 60 * 1000;
const SUCCESS_RATE_WARNING_THRESHOLD = 95;
const SUCCESS_RATE_CRITICAL_THRESHOLD = 90;
const FAILURE_RATE_WARNING_THRESHOLD = 5;
const FAILURE_RATE_CRITICAL_THRESHOLD = 15;

const SCORE_CHART_VISIBLE_LIMIT = 8;

const getScoreSeriesPoints = (rows, deploymentId, currentScore) => {
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

const getScoreBand = (value) => {
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

const emptyConfigMeta = {
  deploymentMap: {},
  orderMap: {},
  virtualMap: {},
  virtualOrder: [],
};

const getPanelKey = (panel) => {
  if (panel === 'dashboard') {
    return 'gateway';
  }
  if (panel === 'legacy') {
    return 'gateway';
  }
  return PANEL_KEYS.has(panel) ? panel : 'gateway';
};

const formatNumber = (value) => {
  const number = Number(value);
  if (!Number.isFinite(number)) {
    return '-';
  }
  return new Intl.NumberFormat('zh-CN').format(number);
};

const formatPercent = (value) => {
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

const formatConcurrency = (row) => {
  const inFlight = Number(row?.in_flight_requests || 0);
  const limit = Number(row?.max_concurrent_requests || 0);
  if (limit > 0) {
    return `${formatNumber(inFlight)} / ${formatNumber(limit)}`;
  }
  return `${formatNumber(inFlight)} / 不限`;
};

const formatTime = (value) => {
  if (!value) {
    return '-';
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return String(value);
  }
  return date.toLocaleString('zh-CN', { hour12: false });
};

const formatInterval = (milliseconds) => {
  if (milliseconds >= 60000) {
    return `${Math.round(milliseconds / 60000)} 分钟`;
  }
  return `${Math.round(milliseconds / 1000)} 秒`;
};

const formatChartTime = (value) => {
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

const formatDuration = (milliseconds) => {
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

const normalizeMetricSamples = (samples) =>
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

const loadMetricSamples = () => {
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

const saveMetricSamples = (samples) => {
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

const calculateWindowRate = (samples, windowMs) => {
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

const getWindowRateNote = (rate) => {
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

const getSuccessRateLevel = (rate) => {
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

const getFailureRateLevel = (rate) => {
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

const isFutureTime = (value, now = Date.now()) => {
  if (!value) {
    return false;
  }
  const time = new Date(value).getTime();
  return Number.isFinite(time) && time > now;
};

const isRecentTime = (value, windowMs, now = Date.now()) => {
  if (!value) {
    return false;
  }
  const time = new Date(value).getTime();
  return Number.isFinite(time) && now - time <= windowMs;
};

const isQuotaExhaustedRow = (row) => {
  const alertType = row?.alert_type;
  if (alertType === 'exhausted' || alertType === 'hard_limit') {
    return true;
  }
  const usedTokens = Number(row?.used_tokens || 0);
  const dailyLimit = Number(row?.daily_limit || 0);
  return dailyLimit > 0 && usedTokens >= dailyLimit;
};

const buildDeploymentMeta = (config) => {
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

const getStatusMeta = (row) => {
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

const getLevelColor = (level) => {
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

const parseMetricLabels = (labelText) => {
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

const parseMetrics = (metricsText) =>
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

const Fallback = () => {
  const { panel } = useParams();
  const navigate = useNavigate();
  const activePanel = getPanelKey(panel);
  const activePanelItem =
    PANEL_ITEMS.find((item) => item.key === activePanel) || PANEL_ITEMS[0];
  const refreshInterval = PANEL_REFRESH_INTERVALS[activePanel] || 15000;
  const refreshHint = `自动每 ${formatInterval(
    refreshInterval
  )} 刷新，点击可立即显示最新数据`;
  const admin = isAdmin();

  const [loading, setLoading] = useState(false);
  const [lastUpdated, setLastUpdated] = useState(null);
  const [statusRows, setStatusRows] = useState([]);
  const [metricsText, setMetricsText] = useState('');
  const [scores, setScores] = useState({});
  const [scoreHistory, setScoreHistory] = useState([]);
  const [alertEvents, setAlertEvents] = useState([]);
  const [switchEvents, setSwitchEvents] = useState([]);
  const [configMeta, setConfigMeta] = useState(emptyConfigMeta);
  const [statusSort, setStatusSort] = useState('config');
  const [actingDeployment, setActingDeployment] = useState('');
  const [guideOpen, setGuideOpen] = useState(false);
  const [summary, setSummary] = useState(null);
  const [metricSamples, setMetricSamples] = useState(loadMetricSamples);

  const markAllAlertsRead = useCallback(async () => {
    try {
      const res = await API.post('/api/fallback/alert/read-all');
      if (res.data?.success) {
        setAlertEvents((prev) => prev.map((e) => ({ ...e, read: true })));
      }
    } catch (e) {
      console.error('mark all alerts read failed:', e);
    }
  }, []);


  const loadConfigMeta = useCallback(async () => {
    const res = await API.get('/api/fallback/editor/config');
    const { success, message, data } = res.data || {};
    if (success === false) {
      throw new Error(message || '加载 fallback 配置失败');
    }
    const meta = buildDeploymentMeta(data);
    setConfigMeta(meta);
    return meta;
  }, []);

  const loadSummary = useCallback(async () => {
    if (!admin) return;
    try {
      const res = await API.get('/api/fallback/summary');
      const { success, data } = res.data || {};
      if (success) {
        setSummary(data);
      }
    } catch (e) {
      // silently fail
    }
  }, [admin]);

  const loadPanel = useCallback(
    async (silent = false) => {
      if (!admin) {
        return;
      }
      if (!silent) {
        setLoading(true);
      }
      try {
        if (activePanel === 'status') {
          const [statusRes] = await Promise.all([
            API.get('/api/fallback/alert/status'),
            loadConfigMeta(),
          ]);
          setStatusRows(
            Array.isArray(statusRes.data?.status) ? statusRes.data.status : []
          );
        } else if (activePanel === 'metrics') {
          const [metricsRes, statusRes, logsRes] = await Promise.all([
            API.get('/metrics', { responseType: 'text' }),
            API.get('/api/fallback/alert/status'),
            API.get('/api/fallback/logs?limit=500'),
            loadConfigMeta(),
          ]);
          const { success, message, data } = logsRes.data || {};
          if (success === false) {
            throw new Error(message || '加载回退事件日志失败');
          }
          setMetricsText(metricsRes.data || '');
          setStatusRows(
            Array.isArray(statusRes.data?.status) ? statusRes.data.status : []
          );
          setSwitchEvents(Array.isArray(data) ? data : []);
        } else if (activePanel === 'scores') {
          const [scoresRes] = await Promise.all([
            API.get('/api/fallback/sort/scores'),
            loadConfigMeta(),
          ]);
          const historyRes = await API.get('/api/fallback/sort/history?limit=300');
          const { success, message, data } = historyRes.data || {};
          if (success === false) {
            throw new Error(message || '加载排序分数历史失败');
          }
          setScores(scoresRes.data?.scores || {});
          setScoreHistory(Array.isArray(data) ? data : []);
        } else if (activePanel === 'alerts') {
          const res = await API.get('/api/fallback/alert/history?limit=100');
          const { success, message, data } = res.data || {};
          if (success === false) {
            throw new Error(message || '加载告警历史失败');
          }
          setAlertEvents(Array.isArray(data) ? data : []);
        } else if (activePanel === 'logs') {
          const res = await API.get('/api/fallback/logs?limit=100');
          const { success, message, data } = res.data || {};
          if (success === false) {
            throw new Error(message || '加载回退事件日志失败');
          }
          setSwitchEvents(Array.isArray(data) ? data : []);
        }
        setLastUpdated(new Date());
      } catch (error) {
        if (!silent) {
          showError(error.message || '加载 fallback 面板失败');
        }
      } finally {
        if (!silent) {
          setLoading(false);
        }
      }
    },
    [activePanel, admin, loadConfigMeta]
  );

  useEffect(() => {
    if (!panel || panel !== activePanel) {
      navigate(`/fallback/${activePanel}`, { replace: true });
    }
  }, [activePanel, navigate, panel]);

  useEffect(() => {
    loadPanel().then();
  }, [loadPanel]);

  useEffect(() => {
    loadSummary().then();
  }, [loadSummary]);

  useEffect(() => {
    const timer = window.setInterval(() => {
      loadSummary().then();
    }, 30000);
    return () => window.clearInterval(timer);
  }, [loadSummary]);

  useEffect(() => {
    const timer = window.setInterval(() => {
      loadPanel(true).then();
    }, refreshInterval);
    return () => window.clearInterval(timer);
  }, [loadPanel, refreshInterval]);

  const statusDisplayRows = useMemo(() => {
    const rows = statusRows.map((row, index) => {
      const deploymentId = row.deployment_id || '';
      const deploymentConfig = configMeta.deploymentMap[deploymentId] || {};
      const virtualModels = configMeta.virtualMap[deploymentId] || [];
      return {
        ...deploymentConfig,
        ...row,
        deployment_id: deploymentId,
        real_model: row.real_model || deploymentConfig.real_model || '-',
        virtual_models: virtualModels.join(', ') || '-',
        config_order:
          configMeta.orderMap[deploymentId] === undefined
            ? 100000 + index
            : configMeta.orderMap[deploymentId],
      };
    });

    return rows.sort((left, right) => {
      if (statusSort === 'tokens') {
        const tokenDiff =
          Number(right.used_tokens || 0) - Number(left.used_tokens || 0);
        if (tokenDiff !== 0) {
          return tokenDiff;
        }
      } else if (statusSort === 'model') {
        const modelDiff = String(left.real_model || '').localeCompare(
          String(right.real_model || ''),
          'zh-CN'
        );
        if (modelDiff !== 0) {
          return modelDiff;
        }
      }
      if (left.config_order !== right.config_order) {
        return left.config_order - right.config_order;
      }
      return String(left.deployment_id).localeCompare(
        String(right.deployment_id),
        'zh-CN'
      );
    });
  }, [configMeta, statusRows, statusSort]);

  const metricRows = useMemo(() => parseMetrics(metricsText), [metricsText]);

  const runtimeMetrics = useMemo(() => {
    const metricValue = (name) => {
      const row = metricRows.find((item) => item.name === name);
      return Number.isFinite(row?.numericValue) ? row.numericValue : 0;
    };

    const requests = metricValue('fallback_requests_total');
    const switches = metricValue('fallback_switch_total');
    const failed = metricValue('fallback_failed_total');
    const success = metricValue('fallback_success_total');
    const handled = success + failed;
    const successRate = handled > 0 ? (success / handled) * 100 : null;
    const tokenRows = metricRows
      .filter((row) => row.name === 'deployment_used_tokens')
      .map((row) => ({
        deployment: row.labels.deployment || row.displayName,
        tokens: Number.isFinite(row.numericValue) ? row.numericValue : 0,
      }))
      .sort((left, right) => {
        if (right.tokens !== left.tokens) {
          return right.tokens - left.tokens;
        }
        return left.deployment.localeCompare(right.deployment, 'zh-CN');
      });
    const totalTokens = tokenRows.reduce((sum, row) => sum + row.tokens, 0);
    const maxDeploymentTokens = Math.max(
      1,
      ...tokenRows.map((row) => row.tokens)
    );

    return {
      requests,
      switches,
      success,
      failed,
      successRate,
      tokenRows,
      totalTokens,
      maxDeploymentTokens,
    };
  }, [metricRows]);

  useEffect(() => {
    if (activePanel !== 'metrics' || !String(metricsText || '').trim()) {
      return;
    }

    const now = Date.now();
    const sample = {
      timestamp: now,
      requests: runtimeMetrics.requests,
      switches: runtimeMetrics.switches,
      success: runtimeMetrics.success,
      failed: runtimeMetrics.failed,
    };

    setMetricSamples((previousSamples) => {
      const cutoff = now - METRIC_SAMPLE_RETENTION_MS;
      const retainedSamples = normalizeMetricSamples(previousSamples).filter(
        (item) => item.timestamp >= cutoff
      );
      const lastSample = retainedSamples[retainedSamples.length - 1];
      const sameCounters =
        lastSample &&
        lastSample.requests === sample.requests &&
        lastSample.switches === sample.switches &&
        lastSample.success === sample.success &&
        lastSample.failed === sample.failed;
      const tooSoon = lastSample && sample.timestamp - lastSample.timestamp < 5000;
      const nextSamples =
        sameCounters && tooSoon ? retainedSamples : [...retainedSamples, sample];
      saveMetricSamples(nextSamples);
      return nextSamples;
    });
  }, [
    activePanel,
    metricsText,
    runtimeMetrics.failed,
    runtimeMetrics.requests,
    runtimeMetrics.success,
    runtimeMetrics.switches,
  ]);

  const runtimeHealth = useMemo(() => {
    const now = Date.now();
    const fiveMinuteRate = calculateWindowRate(
      metricSamples,
      FIVE_MINUTES_MS
    );
    const oneHourRate = calculateWindowRate(metricSamples, ONE_HOUR_MS);
    const deploymentMetaById = {
      ...configMeta.deploymentMap,
    };
    statusDisplayRows.forEach((row) => {
      if (row.deployment_id) {
        deploymentMetaById[row.deployment_id] = {
          ...(deploymentMetaById[row.deployment_id] || {}),
          ...row,
        };
      }
    });

    const coolingRows = statusDisplayRows.filter(
      (row) =>
        row.alert_type === 'cooldown' || isFutureTime(row.cooldown_until, now)
    );
    const quotaExhaustedRows = statusDisplayRows.filter(isQuotaExhaustedRow);
    const recentSwitchEvents = switchEvents.filter((event) =>
      isRecentTime(event.created_at, ONE_HOUR_MS, now)
    );

    const deploymentFailures = new Map();
    const modelFailures = new Map();
    const channelFailures = new Map();

    recentSwitchEvents.forEach((event) => {
      const deploymentId =
        event.from_deployment || event.to_deployment || 'unknown';
      const deploymentMeta = deploymentMetaById[deploymentId] || {};
      const channelId =
        deploymentMeta.channel_id === undefined ||
        deploymentMeta.channel_id === null ||
        deploymentMeta.channel_id === ''
          ? ''
          : String(deploymentMeta.channel_id);
      const realModel =
        deploymentMeta.real_model || event.virtual_model || deploymentId;
      const deploymentKey = deploymentId || 'unknown';
      const channelKey = channelId ? `#${channelId}` : '未绑定渠道';
      const modelKey = event.virtual_model || realModel || '未知模型';

      const deploymentItem =
        deploymentFailures.get(deploymentKey) || {
          key: deploymentKey,
          deployment: deploymentId,
          channel: channelKey,
          model: realModel,
          count: 0,
          lastReason: '',
          lastStatusCode: 0,
        };
      deploymentItem.count += 1;
      deploymentItem.lastReason = event.reason || deploymentItem.lastReason;
      deploymentItem.lastStatusCode =
        event.status_code || deploymentItem.lastStatusCode;
      deploymentFailures.set(deploymentKey, deploymentItem);

      const channelItem =
        channelFailures.get(channelKey) || {
          key: channelKey,
          channel: channelKey,
          count: 0,
        };
      channelItem.count += 1;
      channelFailures.set(channelKey, channelItem);

      const modelItem =
        modelFailures.get(modelKey) || {
          key: modelKey,
          model: modelKey,
          count: 0,
        };
      modelItem.count += 1;
      modelFailures.set(modelKey, modelItem);
    });

    const sortFailures = (items) =>
      Array.from(items.values())
        .sort((left, right) => {
          if (right.count !== left.count) {
            return right.count - left.count;
          }
          return String(left.key).localeCompare(String(right.key), 'zh-CN');
        })
        .slice(0, 5);

    const issues = [];
    let level = 'normal';

    const markIssue = (nextLevel, message) => {
      issues.push(message);
      if (nextLevel === 'critical') {
        level = 'critical';
      } else if (level !== 'critical') {
        level = 'warning';
      }
    };

    if (
      fiveMinuteRate.successRate !== null &&
      fiveMinuteRate.successRate < SUCCESS_RATE_CRITICAL_THRESHOLD
    ) {
      markIssue(
        'critical',
        `近 5 分钟成功率低于 ${SUCCESS_RATE_CRITICAL_THRESHOLD}%`
      );
    } else if (
      fiveMinuteRate.successRate !== null &&
      fiveMinuteRate.successRate < SUCCESS_RATE_WARNING_THRESHOLD
    ) {
      markIssue(
        'warning',
        `近 5 分钟成功率低于 ${SUCCESS_RATE_WARNING_THRESHOLD}%`
      );
    }

    if (
      oneHourRate.failureRate !== null &&
      oneHourRate.failureRate >= FAILURE_RATE_CRITICAL_THRESHOLD
    ) {
      markIssue(
        'critical',
        `近 1 小时失败率达到 ${FAILURE_RATE_CRITICAL_THRESHOLD}%`
      );
    } else if (
      oneHourRate.failureRate !== null &&
      oneHourRate.failureRate >= FAILURE_RATE_WARNING_THRESHOLD
    ) {
      markIssue(
        'warning',
        `近 1 小时失败率达到 ${FAILURE_RATE_WARNING_THRESHOLD}%`
      );
    }

    if (quotaExhaustedRows.length > 0) {
      markIssue('critical', `${quotaExhaustedRows.length} 个部署额度耗尽`);
    }
    if (coolingRows.length > 0) {
      markIssue('warning', `${coolingRows.length} 个部署正在冷却`);
    }

    return {
      level,
      title:
        level === 'critical'
          ? '需要处理'
          : level === 'warning'
          ? '需要关注'
          : '运行平稳',
      message:
        issues.length > 0
          ? issues.join('，')
          : '最近窗口内未发现明显 fallback 风险。',
      fiveMinuteRate,
      oneHourRate,
      coolingRows,
      quotaExhaustedRows,
      topDeploymentFailures: sortFailures(deploymentFailures),
      topChannelFailures: sortFailures(channelFailures),
      topModelFailures: sortFailures(modelFailures),
      recentSwitchCount: recentSwitchEvents.length,
    };
  }, [configMeta.deploymentMap, metricSamples, statusDisplayRows, switchEvents]);

  const metricTrendData = useMemo(() => {
    const samples = normalizeMetricSamples(metricSamples);
    if (samples.length < 2) return [];
    const now = Date.now();
    const oneHourAgo = now - ONE_HOUR_MS;
    const recentSamples = samples.filter((s) => s.timestamp >= oneHourAgo);
    if (recentSamples.length < 2) return [];
    const data = [];
    for (let i = 1; i < recentSamples.length; i++) {
      const prev = recentSamples[i - 1];
      const curr = recentSamples[i];
      const requests = Math.max(0, curr.requests - prev.requests);
      const switches = Math.max(0, curr.switches - prev.switches);
      const success = Math.max(0, curr.success - prev.success);
      const failed = Math.max(0, curr.failed - prev.failed);
      const handled = success + failed;
      const successRate = handled > 0 ? Number(((success / handled) * 100).toFixed(1)) : null;
      data.push({
        time: new Date(curr.timestamp).toLocaleTimeString('zh-CN', {
          hour12: false,
          hour: '2-digit',
          minute: '2-digit',
        }),
        requests,
        successRate,
        switches,
      });
    }
    return data;
  }, [metricSamples]);

  const scoreGroups = useMemo(() => {
    const orderMap = new Map(
      configMeta.virtualOrder.map((virtualModel, index) => [
        virtualModel,
        index,
      ])
    );
    return Object.keys(scores).sort((left, right) => {
      const leftOrder = orderMap.has(left) ? orderMap.get(left) : 100000;
      const rightOrder = orderMap.has(right) ? orderMap.get(right) : 100000;
      if (leftOrder !== rightOrder) {
        return leftOrder - rightOrder;
      }
      return left.localeCompare(right, 'zh-CN');
    });
  }, [configMeta.virtualOrder, scores]);

  const scoreTrend = useMemo(() => {
    const rowsByTime = new Map();
    const deploymentSet = new Set();

    (Array.isArray(scoreHistory) ? scoreHistory : []).forEach((event) => {
      const deploymentId = event?.deployment_id;
      if (!deploymentId) {
        return;
      }
      const score = Number(event.score);
      if (!Number.isFinite(score)) {
        return;
      }

      const createdAt = event.created_at || '';
      const date = new Date(createdAt);
      const order = Number.isNaN(date.getTime()) ? 0 : date.getTime();
      const key = createdAt || `${order}:${deploymentId}`;
      if (!rowsByTime.has(key)) {
        rowsByTime.set(key, {
          key,
          order,
          time: formatChartTime(createdAt),
          values: {},
        });
      }

      rowsByTime.get(key).values[deploymentId] = Number(score.toFixed(2));
      deploymentSet.add(deploymentId);
    });

    const rows = Array.from(rowsByTime.values()).sort((left, right) => {
      if (left.order !== right.order) {
        return left.order - right.order;
      }
      return left.key.localeCompare(right.key, 'zh-CN');
    });
    const deployments = Array.from(deploymentSet).sort((left, right) => {
      const leftOrder =
        configMeta.orderMap[left] === undefined ? 100000 : configMeta.orderMap[left];
      const rightOrder =
        configMeta.orderMap[right] === undefined ? 100000 : configMeta.orderMap[right];
      if (leftOrder !== rightOrder) {
        return leftOrder - rightOrder;
      }
      return left.localeCompare(right, 'zh-CN');
    });

    const latestScores = deployments
      .map((deploymentId) => {
        for (let i = rows.length - 1; i >= 0; i -= 1) {
          const value = rows[i].values[deploymentId];
          if (Number.isFinite(value)) {
            return { deploymentId, value };
          }
        }
        return null;
      })
      .filter(Boolean);
    const chartDeployments = latestScores
      .slice()
      .sort((left, right) => right.value - left.value)
      .slice(0, SCORE_CHART_VISIBLE_LIMIT)
      .map((item) => item.deploymentId);
    const scoreValues = latestScores.map((item) => item.value);
    const scoreSummary =
      scoreValues.length > 0
        ? {
            max: Math.max(...scoreValues),
            min: Math.min(...scoreValues),
            avg:
              scoreValues.reduce((sum, value) => sum + value, 0) /
              scoreValues.length,
          }
        : null;

    return { rows, deployments, chartDeployments, latestScores, scoreSummary };
  }, [configMeta.orderMap, scoreHistory]);

  const scoreTrendGroups = useMemo(() => {
    return scoreGroups
      .map((virtualModel) => {
        const groupScores = scores[virtualModel] || {};
        const items = sortScoreItems(
          Object.keys(groupScores)
          .map((deploymentId) => {
            const value = Number(groupScores[deploymentId]);
            return Number.isFinite(value)
              ? {
                  deploymentId,
                  value,
                }
              : null;
          })
          .filter(Boolean)
        )
          .slice(0, SCORE_CHART_VISIBLE_LIMIT);

        return {
          virtualModel,
          items,
        };
      })
      .filter((group) => group.items.length > 0);
  }, [scoreGroups, scores]);

  const runDeploymentAction = async (deploymentId, action) => {
    const actionKey = `${deploymentId}:${action}`;
    setActingDeployment(actionKey);
    try {
      let url = `/api/fallback/deployments/${encodeURIComponent(deploymentId)}`;
      if (action === 'cooldown') {
        url += '/cooldown?duration_seconds=300';
      } else {
        url += '/recover';
      }

      const res = await API.post(url);
      const { success, message } = res.data || {};
      if (success === false) {
        throw new Error(message || '部署状态操作失败');
      }

      await loadPanel(true);
      showSuccess(
        action === 'cooldown' ? '已设置冷却' : '已恢复并重置当前周期额度'
      );
    } catch (error) {
      showError(error.message || '部署状态操作失败');
    } finally {
      setActingDeployment('');
    }
  };

  const exportMetricsCSV = useCallback(() => {
    if (metricRows.length === 0) return;
    const headers = '﻿指标,值\n';
    const csvRows = metricRows.map((row) => `${row.displayName},${row.value}`);
    const csv = headers + csvRows.join('\n');
    const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `fallback-metrics-${new Date().toISOString().slice(0, 19).replace(/[:-]/g, '')}.csv`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
  }, [metricRows]);

  const renderStatusPanel = () => (
    <>
      <FallbackRuntimePanel />
      <div className='fallback-content-toolbar'>
        <div>
          <h2>部署状态</h2>
          <span>当前状态、Token 用量和手动操作</span>
        </div>
        <Dropdown
          selection
          compact
          options={STATUS_SORT_OPTIONS}
          value={statusSort}
          onChange={(_, { value }) => setStatusSort(value)}
        />
      </div>
      <div className='fallback-table-wrap'>
        <Table compact celled selectable={false} striped>
          <Table.Header>
            <Table.Row>
              <Table.HeaderCell>部署</Table.HeaderCell>
              <Table.HeaderCell>模型</Table.HeaderCell>
              <Table.HeaderCell>级别</Table.HeaderCell>
              <Table.HeaderCell>用量</Table.HeaderCell>
              <Table.HeaderCell>Token</Table.HeaderCell>
              <Table.HeaderCell>权重</Table.HeaderCell>
              <Table.HeaderCell>并发</Table.HeaderCell>
              <Table.HeaderCell>状态</Table.HeaderCell>
              <Table.HeaderCell>操作</Table.HeaderCell>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {statusDisplayRows.length === 0 ? (
              <Table.Row>
                <Table.Cell colSpan='9' textAlign='center'>
                  暂无 fallback 部署数据
                </Table.Cell>
              </Table.Row>
            ) : (
              statusDisplayRows.map((row) => {
                const statusMeta = getStatusMeta(row);
                return (
                  <Table.Row
                    key={row.deployment_id}
                    className={`fallback-deploy-row ${
                      row.alert_type === 'cooldown'
                        ? 'cooling'
                        : isQuotaExhaustedRow(row)
                        ? 'quota-exhausted'
                        : ''
                    }`}
                  >
                   <Table.Cell>
                     <strong>{row.deployment_id}</strong>
                     <div className='fallback-muted'>{row.virtual_models}</div>
                   </Table.Cell>
                    <Table.Cell>
                      <strong>{row.deployment_id}</strong>
                    </Table.Cell>
                    <Table.Cell>
                      <span className='fallback-code-text'>
                        {row.real_model}
                      </span>
                    </Table.Cell>
                    <Table.Cell>
                      <Label color={getLevelColor(row.alert_level)}>
                        {row.alert_level || 'normal'}
                      </Label>
                    </Table.Cell>
                    <Table.Cell>{formatPercent(row.usage_percent)}</Table.Cell>
                    <Table.Cell>
                      {formatNumber(row.used_tokens)} /{' '}
                      {formatNumber(row.daily_limit)}
                    </Table.Cell>
                    <Table.Cell>{formatNumber(row.weight || 100)}</Table.Cell>
                    <Table.Cell className='fallback-value-cell'>
                      {formatConcurrency(row)}
                    </Table.Cell>
                    <Table.Cell>
                      <span className='fallback-status-row'>
                        <span
                          className='fallback-status-dot'
                          style={{
                            background:
                              statusMeta.color === 'green'
                                ? '#22c55e'
                                : statusMeta.color === 'orange'
                                ? '#f97316'
                                : statusMeta.color === 'red'
                                ? '#ef4444'
                                : statusMeta.color === 'yellow'
                                ? '#eab308'
                                : '#94a3b8',
                          }}
                        />
                        <span>{statusMeta.text}</span>
                      </span>
                      <div className='fallback-muted'>{statusMeta.note}</div>
                    </Table.Cell>
                    <Table.Cell>
                      <Button.Group size='mini' style={{ minWidth: 120 }}>
                        <Button

                          basic
                          color='orange'
                          title='冷却 5 分钟，不重置额度'
                          loading={
                            actingDeployment ===
                            `${row.deployment_id}:cooldown`
                          }
                          disabled={Boolean(actingDeployment)}
                          onClick={() =>
                            runDeploymentAction(row.deployment_id, 'cooldown')
                          }
                        >
                          <Icon name='pause circle' /> 暂停
                        </Button>
                        <Button

                          basic
                          color='green'
                          title='恢复部署并重置当前周期额度'
                          loading={
                            actingDeployment === `${row.deployment_id}:recover`
                          }
                          disabled={Boolean(actingDeployment)}
                          onClick={() =>
                            runDeploymentAction(row.deployment_id, 'recover')
                          }
                        >
                          <Icon name='undo' /> 恢复
                        </Button>
                      </Button.Group>
                    </Table.Cell>
                  </Table.Row>
                );
              })
            )}
          </Table.Body>
        </Table>
      </div>
    </>
  );

  const renderMetricsPanel = () => (
    <>
      <div className='fallback-content-toolbar'>
        <div>
          <h2>运行数据</h2>
          <span>
            每 30 秒刷新，展示请求量、切换次数、成功失败和 token 消耗。
          </span>
        </div>
      </div>
      <div className='fallback-runtime-grid'>
        <article className='fallback-runtime-card'>
          <span>请求量</span>
          <strong>{formatNumber(runtimeMetrics.requests)}</strong>
          <small>fallback_requests_total</small>
        </article>
        <article className='fallback-runtime-card'>
          <span>切换次数</span>
          <strong>{formatNumber(runtimeMetrics.switches)}</strong>
          <small>
            近 1 小时 {formatNumber(summary?.switch_count || 0)} 次
          </small>
        </article>
        <article className='fallback-runtime-card'>
          <span>成功 / 失败</span>
          <strong>
            {formatNumber(runtimeMetrics.success)} /{' '}
            {formatNumber(runtimeMetrics.failed)}
          </strong>
          <small>
            成功率{' '}
            {runtimeMetrics.successRate === null
              ? '-'
              : formatPercent(runtimeMetrics.successRate)}
          </small>
        </article>
        <article className='fallback-runtime-card'>
          <span>Token 消耗</span>
          <strong>{formatNumber(runtimeMetrics.totalTokens)}</strong>
          <small>{runtimeMetrics.tokenRows.length} 个部署有消耗记录</small>
        </article>
      </div>

      {metricTrendData.length > 0 && (
        <section className='fallback-trend-section'>
          <div className='fallback-runtime-section-head'>
            <h3>趋势图表</h3>
            <span>过去 1 小时聚合趋势</span>
          </div>
          <div className='fallback-trend-grid'>
            <div className='fallback-trend-card'>
              <span className='fallback-trend-label'>请求量</span>
              <ResponsiveContainer width='100%' height={160}>
                <LineChart data={metricTrendData} margin={{ top: 8, right: 8, left: -16, bottom: 0 }}>
                  <CartesianGrid strokeDasharray='3 3' stroke='#e3e8ef' vertical={false} />
                  <XAxis dataKey='time' tick={{ fontSize: 11, fill: '#667085' }} axisLine={false} tickLine={false} />
                  <YAxis tick={{ fontSize: 11, fill: '#667085' }} axisLine={false} tickLine={false} width={36} />
                  <Tooltip />
                  <Line type='monotone' dataKey='requests' stroke='#2563eb' strokeWidth={2} dot={false} activeDot={{ r: 4 }} />
                </LineChart>
              </ResponsiveContainer>
            </div>
            <div className='fallback-trend-card'>
              <span className='fallback-trend-label'>成功率</span>
              <ResponsiveContainer width='100%' height={160}>
                <LineChart data={metricTrendData} margin={{ top: 8, right: 8, left: -16, bottom: 0 }}>
                  <CartesianGrid strokeDasharray='3 3' stroke='#e3e8ef' vertical={false} />
                  <XAxis dataKey='time' tick={{ fontSize: 11, fill: '#667085' }} axisLine={false} tickLine={false} />
                  <YAxis domain={[0, 100]} tick={{ fontSize: 11, fill: '#667085' }} axisLine={false} tickLine={false} width={36} />
                  <Tooltip />
                  <Line type='monotone' dataKey='successRate' stroke='#22c55e' strokeWidth={2} dot={false} activeDot={{ r: 4 }} />
                </LineChart>
              </ResponsiveContainer>
            </div>
            <div className='fallback-trend-card'>
              <span className='fallback-trend-label'>切换次数</span>
              <ResponsiveContainer width='100%' height={160}>
                <LineChart data={metricTrendData} margin={{ top: 8, right: 8, left: -16, bottom: 0 }}>
                  <CartesianGrid strokeDasharray='3 3' stroke='#e3e8ef' vertical={false} />
                  <XAxis dataKey='time' tick={{ fontSize: 11, fill: '#667085' }} axisLine={false} tickLine={false} />
                  <YAxis tick={{ fontSize: 11, fill: '#667085' }} axisLine={false} tickLine={false} width={36} />
                  <Tooltip />
                  <Line type='monotone' dataKey='switches' stroke='#f59e0b' strokeWidth={2} dot={false} activeDot={{ r: 4 }} />
                </LineChart>
              </ResponsiveContainer>
            </div>
          </div>
        </section>
      )}

      <section className={`fallback-health-panel ${runtimeHealth.level}`}>
        <div>
          <span className='fallback-health-label'>健康判断</span>
          <h3>{runtimeHealth.title}</h3>
          <p>{runtimeHealth.message}</p>
        </div>
        <div className='fallback-health-meta'>
          <span>近 1 小时切换 {formatNumber(runtimeHealth.recentSwitchCount)} 次</span>
          <span>采样点 {formatNumber(metricSamples.length)}</span>
        </div>
      </section>

      <div className='fallback-runtime-grid fallback-health-grid'>
        <article
          className={`fallback-runtime-card ${getSuccessRateLevel(
            runtimeHealth.fiveMinuteRate
          )}`}
        >
          <span>最近 5 分钟成功率</span>
          <strong>{formatPercent(runtimeHealth.fiveMinuteRate.successRate)}</strong>
          <small>{getWindowRateNote(runtimeHealth.fiveMinuteRate)}</small>
        </article>
        <article
          className={`fallback-runtime-card ${getFailureRateLevel(
            runtimeHealth.oneHourRate
          )}`}
        >
          <span>最近 1 小时失败率</span>
          <strong>{formatPercent(runtimeHealth.oneHourRate.failureRate)}</strong>
          <small>{getWindowRateNote(runtimeHealth.oneHourRate)}</small>
        </article>
        <article
          className={`fallback-runtime-card ${
            runtimeHealth.coolingRows.length > 0 ? 'warning' : 'normal'
          }`}
        >
          <span>当前被冷却部署</span>
          <strong>{formatNumber(runtimeHealth.coolingRows.length)}</strong>
          <small>
            {runtimeHealth.coolingRows.length > 0
              ? runtimeHealth.coolingRows
                  .map((row) => row.deployment_id)
                  .slice(0, 3)
                  .join('、')
              : '暂无冷却部署'}
          </small>
        </article>
        <article
          className={`fallback-runtime-card ${
            runtimeHealth.quotaExhaustedRows.length > 0 ? 'critical' : 'normal'
          }`}
        >
          <span>额度耗尽部署</span>
          <strong>{formatNumber(runtimeHealth.quotaExhaustedRows.length)}</strong>
          <small>
            {runtimeHealth.quotaExhaustedRows.length > 0
              ? runtimeHealth.quotaExhaustedRows
                  .map((row) => row.deployment_id)
                  .slice(0, 3)
                  .join('、')
              : '暂无耗尽部署'}
          </small>
        </article>
      </div>

      {runtimeHealth.topDeploymentFailures.length > 0 && (
        <section className='fallback-runtime-section'>
          <div className='fallback-runtime-section-head'>
            <h3>Top 3 失败模型</h3>
            <span>当前失败率最高的 3 个部署</span>
          </div>
          <div className='fallback-top3-grid'>
            {runtimeHealth.topDeploymentFailures.slice(0, 3).map((item, index) => {
              const rankClass = `rank-${index + 1}`;
              return (
                <div className={`fallback-top3-card ${rankClass}`} key={item.key}>
                  <span className='fallback-top3-badge'>{index + 1}</span>
                  <div className='fallback-top3-body'>
                    <strong>{item.deployment}</strong>
                    <span>{item.model}</span>
                    <em>失败 {formatNumber(item.count)} 次</em>
                  </div>
                </div>
              );
            })}
          </div>
        </section>
      )}

      <section className='fallback-runtime-section fallback-health-section'>
        <div className='fallback-runtime-section-head'>
          <h3>Top 失败模型/渠道</h3>
          <span>近 1 小时切换日志聚合，最多显示前 5 项</span>
        </div>
        {runtimeHealth.recentSwitchCount === 0 ? (
          <Message>近 1 小时暂无切换失败记录</Message>
        ) : (
          <div className='fallback-health-lists'>
            <div className='fallback-health-list critical'>
              <h4>失败部署 / 渠道</h4>
              {runtimeHealth.topDeploymentFailures.map((item) => (
                <div className='fallback-health-row' key={item.key}>
                  <div>
                    <strong>{item.deployment}</strong>
                    <span>
                      {item.channel} · {item.model}
                    </span>
                    {item.lastReason && <em>{item.lastReason}</em>}
                  </div>
                  <Label color={item.lastStatusCode >= 500 ? 'red' : 'yellow'}>
                    {formatNumber(item.count)} 次
                  </Label>
                </div>
              ))}
            </div>
            <div className='fallback-health-list warning'>
              <h4>失败虚拟模型</h4>
              {runtimeHealth.topModelFailures.map((item) => (
                <div className='fallback-health-row' key={item.key}>
                  <div>
                    <strong>{item.model}</strong>
                    <span>触发切换失败</span>
                  </div>
                  <Label color='orange'>{formatNumber(item.count)} 次</Label>
                </div>
              ))}
            </div>
            <div className='fallback-health-list info'>
              <h4>失败渠道</h4>
              {runtimeHealth.topChannelFailures.map((item) => (
                <div className='fallback-health-row' key={item.key}>
                  <div>
                    <strong>{item.channel}</strong>
                    <span>按部署当前渠道映射聚合</span>
                  </div>
                  <Label color='teal'>{formatNumber(item.count)} 次</Label>
                </div>
              ))}
            </div>
          </div>
        )}
      </section>

      <section className='fallback-runtime-section'>
        <div className='fallback-runtime-section-head'>
          <h3>部署 token 消耗</h3>
          <span>来自 deployment_used_tokens</span>
        </div>
        {runtimeMetrics.tokenRows.length === 0 ? (
          <Message>暂无 token 消耗数据</Message>
        ) : (
          <div className='fallback-runtime-token-list'>
            {runtimeMetrics.tokenRows.map((row) => (
              <div className='fallback-runtime-token-row' key={row.deployment}>
                <div>
                  <strong>{row.deployment}</strong>
                  <span>{formatNumber(row.tokens)} tokens</span>
                </div>
                <div className='fallback-runtime-token-track'>
                  <span
                    style={{
                      width: `${Math.max(
                        3,
                        (row.tokens / runtimeMetrics.maxDeploymentTokens) * 100
                      )}%`,
                    }}
                  />
                </div>
              </div>
            ))}
          </div>
        )}
      </section>

      <details className='fallback-raw-block'>
        <summary>📊 原始指标</summary>
        <div className='fallback-raw-metrics-note'>
          Prometheus 文本解析结果
        </div>
        <div className='fallback-table-wrap'>
          <Table compact celled striped>
            <Table.Header>
              <Table.Row>
                <Table.HeaderCell>指标</Table.HeaderCell>
                <Table.HeaderCell>值</Table.HeaderCell>
              </Table.Row>
            </Table.Header>
            <Table.Body>
              {metricRows.length === 0 ? (
                <Table.Row>
                  <Table.Cell colSpan='2' textAlign='center'>
                    暂无指标数据
                  </Table.Cell>
                </Table.Row>
              ) : (
                metricRows.map((row) => (
                  <Table.Row key={row.key}>
                    <Table.Cell>
                      <code>{row.displayName}</code>
                    </Table.Cell>
                    <Table.Cell className='fallback-value-cell'>
                      {row.value || '-'}
                    </Table.Cell>
                  </Table.Row>
                ))
              )}
            </Table.Body>
          </Table>
        </div>
        <div className='fallback-raw-metrics-note'>
          Prometheus 原始文本
        </div>
        <pre>{metricsText || '暂无指标数据'}</pre>
      </details>

      <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: 14 }}>
        <Button basic size='small' onClick={exportMetricsCSV}>
          <Icon name='download' /> 导出 CSV
        </Button>
      </div>
    </>
  );

  const renderScoresPanel = () => (
    <>
      <div className='fallback-content-toolbar'>
        <div>
          <h2>排序分数</h2>
          <span>每个虚拟模型内按当前智能排序分数从高到低展示。</span>
        </div>
        <Button
          basic
          icon
          labelPosition='left'
          size='small'
          loading={loading}
          disabled={loading}
          onClick={() => loadPanel(true)}
        >
          <Icon name='refresh' />
          刷新
        </Button>
      </div>
      <Card fluid className='fallback-score-trend-card'>
        <Card.Content>
          <div className='fallback-score-card-head'>
            <div className='fallback-score-card-title'>
              <Card.Header>分数排序</Card.Header>
              {scoreTrend.rows.length > 0 && (
                <span>
                  按综合得分从高到低排序
                </span>
              )}
            </div>
            {scoreTrend.scoreSummary && (
              <div className='fallback-score-stats'>
                <span>最高 {scoreTrend.scoreSummary.max.toFixed(1)}</span>
                <span>平均 {scoreTrend.scoreSummary.avg.toFixed(1)}</span>
                <span>最低 {scoreTrend.scoreSummary.min.toFixed(1)}</span>
              </div>
            )}
          </div>
          {scoreTrendGroups.length === 0 ? (
            <Message>暂无排序分数历史</Message>
          ) : (
            <div className='fallback-score-trend-rank'>
              {scoreTrendGroups.map((group) => (
                <section
                  className='fallback-score-trend-group'
                  key={group.virtualModel}
                >
                  <div className='fallback-score-trend-group-head'>
                    <strong>{group.virtualModel}</strong>
                    <span>{formatNumber(group.items.length)} 个部署</span>
                  </div>
                  {group.items.map((item, index) => {
                    const deltaMeta = getScoreDeltaMeta(
                      scoreTrend.rows,
                      item.deploymentId
                    );
                    const series = getScoreSeriesPoints(
                      scoreTrend.rows,
                      item.deploymentId,
                      item.value
                    );
                    const rankClass =
                      index === 0
                        ? 'gold'
                        : index === 1
                        ? 'silver'
                        : index === 2
                        ? 'bronze'
                        : 'normal';

                    return (
                      <article
                        className={`fallback-score-trend-row ${
                          index === 0 ? 'is-top' : ''
                        }`}
                        data-score-band={getScoreBand(item.value)}
                        key={`${group.virtualModel}:${item.deploymentId}`}
                      >
                        <span className={`fallback-rank-badge ${rankClass}`}>
                          {String(index + 1).padStart(2, '0')}
                        </span>
                        <div className='fallback-score-trend-main'>
                          <div className='fallback-score-trend-name'>
                            <strong title={item.deploymentId}>
                              {item.deploymentId}
                            </strong>
                            {index === 0 && (
                              <span className='fallback-score-rank-top'>Top</span>
                            )}
                          </div>
                          <div className='fallback-score-trend-track'>
                            <span
                              style={{
                                width: `${clampScore(item.value)}%`,
                              }}
                            />
                          </div>
                        </div>
                        <div className='fallback-score-trend-meta'>
                          <strong>{item.value.toFixed(1)}</strong>
                          <span className={deltaMeta?.direction || 'flat'}>
                            <Icon name={deltaMeta?.icon || 'minus'} />
                            {deltaMeta?.text || '暂无'}
                          </span>
                          <em>{series.length} 点</em>
                        </div>
                      </article>
                    );
                  })}
                </section>
              ))}
            </div>
          )}
        </Card.Content>
      </Card>
    </>
  );

  const renderAlertsPanel = () => (
    <>
      <div className='fallback-content-toolbar'>
        <div>
          <h2>告警历史</h2>
          <span>记录限额、冷却、耗尽和恢复事件。</span>
        </div>
        <div>
          <Button size='small' onClick={markAllAlertsRead}>
            <Icon name='checkmark' /> 全部标为已读
          </Button>
        </div>
      </div>
      <div className='fallback-table-wrap'>
        <Table compact celled striped>
          <Table.Header>
            <Table.Row>
              <Table.HeaderCell>时间</Table.HeaderCell>
              <Table.HeaderCell>部署</Table.HeaderCell>
              <Table.HeaderCell>级别</Table.HeaderCell>
              <Table.HeaderCell>类型</Table.HeaderCell>
              <Table.HeaderCell>Token</Table.HeaderCell>
              <Table.HeaderCell>用量</Table.HeaderCell>
              <Table.HeaderCell>消息</Table.HeaderCell>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {alertEvents.length === 0 ? (
              <Table.Row>
                <Table.Cell colSpan='7' textAlign='center'>
                  暂无告警历史
                </Table.Cell>
              </Table.Row>
            ) : (
              alertEvents.map((event) => (
                <Table.Row key={event.id || `${event.created_at}:${event.deployment_id}`}>
                  <Table.Cell>{formatTime(event.created_at)}</Table.Cell>
                  <Table.Cell>
                    <a href={`/fallback/status?highlight=${event.deployment_id}`}
                       className='fallback-deployment-link'
                       title='查看部署状态'>
                      <strong>{event.deployment_id}</strong>
                    </a>
                  </Table.Cell>
                  <Table.Cell>
                    <Label color={getLevelColor(event.level)}>
                      {event.level || '-'}
                    </Label>
                  </Table.Cell>
                  <Table.Cell>
                    <code>{event.type || '-'}</code>
                  </Table.Cell>
                  <Table.Cell>
                    {event.daily_limit > 0
                      ? `${formatNumber(event.used_tokens)} / ${formatNumber(
                          event.daily_limit
                        )}`
                      : formatNumber(event.used_tokens)}
                  </Table.Cell>
                  <Table.Cell>{formatPercent(event.percentage)}</Table.Cell>
                  <Table.Cell>{event.message || '-'}</Table.Cell>
                </Table.Row>
              ))
            )}
          </Table.Body>
        </Table>
      </div>
    </>
  );

  const renderLogsPanel = () => (
    <>
      <div className='fallback-content-toolbar'>
        <div>
          <h2>回退事件日志</h2>
          <span>记录最近的部署切换、原因和请求耗时。</span>
        </div>
      </div>
      <Message info className='fallback-log-scope-note'>
        这里展示的是 fallback 业务事件：只有发生部署切换时才会记录。程序启动、数据库、Redis
        等系统运行日志请看服务日志文件。
      </Message>
      <div className='fallback-table-wrap'>
        <Table compact celled striped>
          <Table.Header>
            <Table.Row>
              <Table.HeaderCell>时间</Table.HeaderCell>
              <Table.HeaderCell>虚拟模型</Table.HeaderCell>
              <Table.HeaderCell>切换</Table.HeaderCell>
              <Table.HeaderCell>原因</Table.HeaderCell>
              <Table.HeaderCell>状态码</Table.HeaderCell>
              <Table.HeaderCell>耗时</Table.HeaderCell>
              <Table.HeaderCell>请求 ID</Table.HeaderCell>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {switchEvents.length === 0 ? (
              <Table.Row>
                <Table.Cell colSpan='7' textAlign='center'>
                  暂无回退切换事件
                </Table.Cell>
              </Table.Row>
            ) : (
              switchEvents.map((event) => (
                <Table.Row key={event.id || `${event.created_at}:${event.request_id}`}>
                  <Table.Cell>{formatTime(event.created_at)}</Table.Cell>
                  <Table.Cell>
                    <strong>{event.virtual_model || '-'}</strong>
                  </Table.Cell>
                  <Table.Cell>
                    <strong>{event.from_deployment || '-'}</strong>
                    <span className='fallback-arrow'>-&gt;</span>
                    <strong>{event.to_deployment || '-'}</strong>
                  </Table.Cell>
                  <Table.Cell>{translateFallbackReason(event.reason)}</Table.Cell>
                  <Table.Cell>
                    <Label
                      color={
                        event.status_code >= 500
                          ? 'red'
                          : event.status_code >= 400
                          ? 'yellow'
                          : 'green'
                      }
                    >
                      {event.status_code || '-'}
                    </Label>
                  </Table.Cell>
                  <Table.Cell>
                    {event.duration_ms > 0 ? `${event.duration_ms}ms` : '-'}
                  </Table.Cell>
                  <Table.Cell>
                    <code>{event.request_id || '-'}</code>
                  </Table.Cell>
                </Table.Row>
              ))
            )}
          </Table.Body>
        </Table>
      </div>
    </>
  );

  const renderActivePanel = () => {
    if (loading && activePanel !== 'gateway' && activePanel !== 'free-pool') {
      return (
        <div className='fallback-loading'>
          <Loader active inline='centered' />
        </div>
      );
    }

    switch (activePanel) {
      case 'free-pool':
        return <FreeModelPool />;
      case 'gateway':
        return <ModelEditor />;
      case 'metrics':
        return renderMetricsPanel();
      case 'scores':
        return renderScoresPanel();
      case 'alerts':
        return renderAlertsPanel();
      case 'logs':
        return renderLogsPanel();
      default:
        return renderStatusPanel();
    }
  };


  const renderSummaryBar = () => {
    if (!summary) return null;
    const parts = [];
    if (summary.switch_count > 0) {
      parts.push(`过去 1 小时内：${summary.switch_count} 次回退切换`);
    }
    const rateLimitedItems = (summary.rate_limited || [])
      .filter((item) => item.count > 0)
      .map((item) => `${item.deployment_id} 被限流 ${item.count} 次`);
    parts.push(...rateLimitedItems);
    const coolingDownItems = (summary.cooling_down || [])
      .map((depId) => `${depId} 冷却中`);
    parts.push(...coolingDownItems);

    if (parts.length === 0) return null;

    const hasIssue = summary.switch_count > 0 || coolingDownItems.length > 0;

    return (
      <div className={`fallback-summary-bar ${hasIssue ? 'warning' : 'info'}`}>
        <span className='fallback-summary-icon'>{hasIssue ? '⚠️' : '✅'}</span>
        <span className='fallback-summary-text'>{parts.join('，')}</span>
      </div>
    );
  };

  if (!admin) {
    return (
      <div className='fallback-page'>
        <Message warning>需要管理员权限才能查看 fallback 面板。</Message>
      </div>
    );
  }

  return (
    <div className='fallback-page'>
      <section className='fallback-guide-panel' id='fallback-guide'>
        <div className='fallback-guide-head'>
          <div>
            <h2>CCT API Fallback 快速说明</h2>
            <p>
              给第一次接触这个项目的人看的配置说明：这里列出新增能力、配置位置和日常查看入口。
            </p>
          </div>
          <Button
            type='button'
            basic
            size='small'
            aria-expanded={guideOpen}
            aria-controls='fallback-guide-content'
            onClick={() => setGuideOpen((open) => !open)}
          >
            {guideOpen ? '收起说明' : '首次配置看这里'}
            <Icon name={guideOpen ? 'angle up' : 'arrow right'} />
          </Button>
        </div>
        {guideOpen && (
          <div className='fallback-guide-grid' id='fallback-guide-content'>
            {GUIDE_SECTIONS.map((section) => (
              <article className='fallback-guide-card' key={section.title}>
                <span className='fallback-guide-icon'>
                  <Icon name={section.icon} />
                </span>
                <div>
                  <h3>{section.title}</h3>
                  <ul>
                    {section.items.map((item) => (
                      <li key={item}>{item}</li>
                    ))}
                  </ul>
                </div>
              </article>
            ))}
          </div>
        )}
      </section>

      {renderSummaryBar()}

      <div className='fallback-page-header'>
        <div>
          <h1>Fallback 面板</h1>
          <p>{activePanelItem.description}</p>
          <div className='fallback-page-kicker'>
            <span>{activePanelItem.title}</span>
          </div>
        </div>
        <div className='fallback-header-actions'>
          <span>
            最后刷新：
            {lastUpdated ? formatTime(lastUpdated) : '-'}
          </span>
          <span>自动刷新：{formatInterval(refreshInterval)}</span>
          <Popup
            content='功能说明'
            position='bottom center'
            trigger={
              <Button
                as='a'
                href='#fallback-guide'
                basic
                icon
                size='small'
                className='fallback-help-trigger'
                aria-label='功能说明'
                onClick={() => setGuideOpen(true)}
              >
                <Icon name='hand pointer outline' />
              </Button>
            }
          />
          <Button
            basic
            icon
            size='small'
            title={refreshHint}
            onClick={() => loadPanel(true)}
          >
            <Icon name='refresh' />
          </Button>
        </div>
      </div>

      <nav className='fallback-panel-grid'>
        {PANEL_ITEMS.map((item) => (
          <Link
            key={item.key}
            to={`/fallback/${item.key}`}
            className={`fallback-nav-card ${
              activePanel === item.key ? 'active' : ''
            }`}
            style={{ '--panel-accent': item.accent }}
            title={item.description}
          >
            <span className='fallback-nav-icon'>
              <Icon name={item.icon} />
            </span>
            <span className='fallback-nav-content'>
              <span className='fallback-nav-top'>
                <strong>{item.title}</strong>
                <span className='fallback-nav-refresh-badge'>每 {formatInterval(PANEL_REFRESH_INTERVALS[item.key])}</span>
              </span>
            </span>
          </Link>
        ))}
      </nav>

      <section className='fallback-content-panel'>{renderActivePanel()}</section>
    </div>
  );
};

export default Fallback;


























































































