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

const MetricsPanel = ({
  runtimeMetrics,
  metricTrendData,
  runtimeHealth,
  metricSamples,
  metricsText,
  metricRows,
  summary,
  exportMetricsCSV,
}) => (
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

export default MetricsPanel;
