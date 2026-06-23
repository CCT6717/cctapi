// ============================================================
// useFallbackPage.js — Fallback 页面状态管理 hook
// ============================================================

import { useCallback, useEffect, useMemo, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { API, isAdmin, showError, showSuccess } from '../../../helpers';
import {
  buildDeploymentMeta,
  calculateWindowRate,
  emptyConfigMeta,
  FIVE_MINUTES_MS,
  formatChartTime,
  getPanelKey,
  isFutureTime,
  isQuotaExhaustedRow,
  isRecentTime,
  loadMetricSamples as loadMetricSamplesHelper,
  normalizeMetricSamples,
  ONE_HOUR_MS,
  PANEL_REFRESH_INTERVALS,
  parseMetrics,
  saveMetricSamples as saveMetricSamplesHelper,
  SCORE_CHART_VISIBLE_LIMIT,
  sortScoreItems,
  SUCCESS_RATE_CRITICAL_THRESHOLD,
  SUCCESS_RATE_WARNING_THRESHOLD,
  FAILURE_RATE_CRITICAL_THRESHOLD,
  FAILURE_RATE_WARNING_THRESHOLD,
  METRIC_SAMPLE_RETENTION_MS,
} from '../utils/fallbackHelpers';
import { clampScore, sortScoreItems as sortScoreItemsFn } from '../scoreUtils';

export const useFallbackPage = () => {
  const { panel } = useParams();
  const navigate = useNavigate();
  const activePanel = getPanelKey(panel);
  const refreshInterval = PANEL_REFRESH_INTERVALS[activePanel] || 15000;
  const admin = isAdmin();

  // ---- State ----
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
  const [metricSamples, setMetricSamples] = useState(loadMetricSamplesHelper);

  // ---- Data Loading ----
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
    const metricRows = parseMetrics(metricsText);
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
  }, [metricsText]);

  // ---- Effects ----
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

  // ---- Computed Values ----
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
      saveMetricSamplesHelper(nextSamples);
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
        const items = sortScoreItemsFn(
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

  // ---- Return ----
  return {
    // Router
    panel,
    navigate,
    activePanel,

    // State
    loading,
    lastUpdated,
    statusRows,
    metricsText,
    scores,
    scoreHistory,
    alertEvents,
    switchEvents,
    configMeta,
    statusSort,
    actingDeployment,
    guideOpen,
    summary,
    metricSamples,

    // Setters
    setStatusSort,
    setGuideOpen,

    // Computed
    statusDisplayRows,
    metricRows,
    runtimeMetrics,
    runtimeHealth,
    metricTrendData,
    scoreGroups,
    scoreTrend,
    scoreTrendGroups,

    // Actions
    loadPanel,
    loadSummary,
    markAllAlertsRead,
    runDeploymentAction,
    exportMetricsCSV,

    // Meta
    admin,
    refreshInterval,
  };
};
