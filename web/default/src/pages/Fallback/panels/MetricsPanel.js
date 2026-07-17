// ============================================================
// MetricsPanel.js — Fallback 运行数据面板
// ============================================================

import React from 'react';
import { Button, Icon, Label, Message, Table } from 'semantic-ui-react';
import {
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts';
import {
  formatNumber,
  formatPercent,
  getFailureRateLevel,
  getSuccessRateLevel,
  getWindowRateNote,
} from '../utils/fallbackHelpers';
import { useAttemptObservability } from '../hooks/useAttemptObservability';

const ATTEMPT_OUTCOME_LABELS = {
  success: '成功',
  failure: '失败',
  model_rate_limited: '模型限速',
  non_fallbackable: '不可回退',
  model_capability_false_positive: '模型能力不匹配',
  skipped_unavailable: '不可用跳过',
  skipped_quota: '配额跳过',
  skipped_cooldown: '冷却跳过',
};

const ATTEMPT_CATEGORY_LABELS = {
  none: '无错误',
  client: '客户端错误',
  quota: '配额',
  rate_limit: '限速',
  temporary: '临时错误',
  model_access: '模型访问',
  unknown: '未知类别',
};

const getAttemptOutcomeLabel = (outcome) =>
  ATTEMPT_OUTCOME_LABELS[outcome] || '未知结果';

const getAttemptCategoryLabel = (category) =>
  ATTEMPT_CATEGORY_LABELS[category] || '未知类别';

const getAttemptOutcomeTone = (outcome) => {
  if (outcome === 'success') return 'success';
  if (outcome?.startsWith('skipped_')) return 'skipped';
  return 'failure';
};

const encodeAttemptKeyPart = (value) => {
  const text = String(value ?? '');
  return `${text.length}:${text}`;
};

const getAttemptChainKeyBase = (chain) =>
  [
    chain.request_id,
    chain.virtual_model,
    chain.started_at || chain.steps?.[0]?.created_at,
  ]
    .map(encodeAttemptKeyPart)
    .join('');

const AttemptAggregateList = ({ title, items, valueField, emptyText }) => (
  <article className='fallback-attempt-diagnostic-card'>
    <h4>{title}</h4>
    {items.length === 0 ? (
      <span className='fallback-attempt-empty'>{emptyText}</span>
    ) : (
      <div className='fallback-attempt-diagnostic-list'>
        {items.map((item) => (
          <div className='fallback-attempt-diagnostic-row' key={item.key}>
            <span>{item[valueField] || '-'}</span>
            <Label basic size='mini'>
              {formatNumber(item.count)} 次
            </Label>
          </div>
        ))}
      </div>
    )}
  </article>
);

const MetricsPanel = ({
  runtimeMetrics,
  metricTrendData,
  runtimeHealth,
  metricSamples,
  metricsText,
  metricRows,
  summary,
  exportMetricsCSV,
}) => {
  const {
    data: attemptData,
    error: attemptError,
    loading: attemptLoading,
  } = useAttemptObservability();
  const skippedOutcomes = (attemptData?.outcomes || []).filter((item) =>
    item.outcome?.startsWith('skipped_')
  );
  const chainKeyOccurrences = new Map();

  return (
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

    <section className='fallback-runtime-section fallback-attempt-section'>
      <div className='fallback-runtime-section-head'>
        <h3>精准失败诊断</h3>
        <span>近 1 小时真实尝试聚合，本地跳过单独统计</span>
      </div>
      {attemptError && <Message warning>{attemptError}</Message>}
      {attemptLoading && !attemptData ? (
        <Message>正在加载精准尝试数据...</Message>
      ) : !attemptData ? (
        !attemptError && <Message>精准尝试数据暂时不可用</Message>
      ) : (
        <>
          <div className='fallback-attempt-summary'>
            <article className='fallback-attempt-stat failure'>
              <div>
                <span>真实上游失败</span>
                <small>已实际发往上游的失败尝试</small>
              </div>
              <strong>{formatNumber(attemptData.failure_event_count)}</strong>
            </article>
            <article className='fallback-attempt-stat skipped'>
              <div>
                <span>本地跳过</span>
                <small>
                  {skippedOutcomes.length === 0
                    ? '未调用上游的本地路由判断'
                    : skippedOutcomes
                        .map(
                          (item) =>
                            `${getAttemptOutcomeLabel(item.outcome)} ${formatNumber(
                              item.count
                            )} 次`
                        )
                        .join(' · ')}
                </small>
              </div>
              <strong>{formatNumber(attemptData.skip_event_count)}</strong>
            </article>
          </div>

          <div className='fallback-attempt-diagnostic-grid'>
            <AttemptAggregateList
              title='失败部署'
              items={attemptData.top_deployments || []}
              valueField='deployment_id'
              emptyText='暂无真实部署失败'
            />
            <AttemptAggregateList
              title='失败提供商'
              items={attemptData.top_providers || []}
              valueField='provider'
              emptyText='暂无提供商失败'
            />
            <AttemptAggregateList
              title='失败真实模型'
              items={attemptData.top_models || []}
              valueField='real_model'
              emptyText='暂无真实模型失败'
            />
            <article className='fallback-attempt-diagnostic-card'>
              <h4>错误类别</h4>
              {(attemptData.error_categories || []).length === 0 ? (
                <span className='fallback-attempt-empty'>暂无错误类别</span>
              ) : (
                <div className='fallback-attempt-diagnostic-list'>
                  {attemptData.error_categories.map((item) => (
                    <div
                      className='fallback-attempt-diagnostic-row'
                      key={item.key}
                    >
                      <span>{getAttemptCategoryLabel(item.category)}</span>
                      <Label basic size='mini'>
                        {formatNumber(item.count)} 次
                      </Label>
                    </div>
                  ))}
                </div>
              )}
            </article>
          </div>

          <div className='fallback-attempt-chain-head'>
            <h4>最近请求链路</h4>
            <span>当前进程内最近完成或进行中的路由尝试</span>
          </div>
          {(attemptData.recent_chains || []).length === 0 ? (
            <Message>暂无最近请求链路</Message>
          ) : (
            <div className='fallback-attempt-chain-list'>
              {attemptData.recent_chains.map((chain, chainIndex) => {
                const chainKeyBase = getAttemptChainKeyBase(chain);
                const chainOccurrence = chainKeyOccurrences.get(chainKeyBase) || 0;
                chainKeyOccurrences.set(chainKeyBase, chainOccurrence + 1);
                const chainKey = `${chainKeyBase}#${chainOccurrence}`;
                const chainAccessibleName = `请求链路 ${
                  chain.request_id || '未提供请求 ID'
                }，${chain.virtual_model || '未知虚拟模型'}，第 ${
                  chainIndex + 1
                } 条`;
                return (
                  <article
                    aria-label={chainAccessibleName}
                    className='fallback-attempt-chain-card'
                    key={chainKey}
                  >
                  <div className='fallback-attempt-chain-title'>
                    <div>
                      <strong>{chain.virtual_model || '-'}</strong>
                      <span>{chain.request_id || '-'}</span>
                    </div>
                    <Label basic size='mini'>
                      {formatNumber(chain.steps?.length || 0)} 步
                    </Label>
                  </div>
                  <ol className='fallback-attempt-steps'>
                    {(chain.steps || []).map((step, index) => {
                      const outcomeTone = getAttemptOutcomeTone(step.outcome);
                      const stepMeta = [step.provider, step.deployment_id]
                        .filter(Boolean)
                        .join(' · ');
                      const stepDetails = [
                        step.status_code ? `HTTP ${step.status_code}` : '',
                        step.error_category && step.error_category !== 'none'
                          ? getAttemptCategoryLabel(step.error_category)
                          : '',
                        Number.isFinite(step.duration_ms)
                          ? `${formatNumber(step.duration_ms)} ms`
                          : '',
                      ]
                        .filter(Boolean)
                        .join(' · ');
                      return (
                        <li
                          className={`fallback-attempt-step ${outcomeTone}`}
                          key={`${step.upstream_attempt_index}-${index}`}
                        >
                          <span className='fallback-attempt-step-index'>
                            {index + 1}
                          </span>
                          <div className='fallback-attempt-step-body'>
                            <div>
                              <strong>{step.real_model || '-'}</strong>
                              <Label
                                basic
                                size='mini'
                                color={
                                  outcomeTone === 'success'
                                    ? 'green'
                                    : outcomeTone === 'skipped'
                                    ? 'orange'
                                    : 'red'
                                }
                              >
                                {getAttemptOutcomeLabel(step.outcome)}
                              </Label>
                            </div>
                            {stepMeta && <span>{stepMeta}</span>}
                            {stepDetails && <small>{stepDetails}</small>}
                          </div>
                        </li>
                      );
                    })}
                  </ol>
                  </article>
                );
              })}
            </div>
          )}
        </>
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
};

export default MetricsPanel;
