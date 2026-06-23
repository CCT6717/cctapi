import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { Button, Icon, Label, Loader, Message, Table } from 'semantic-ui-react';
import { API, showError, showSuccess } from '../helpers';
import './FallbackRuntimePanel.css';

// 三层网关只读状态面板：展示 cct/high|low|free 虚拟模型、deployment 运行状态（四维限额）、
// 健康状态。保留两个低风险运维按钮：手动健康检查、手动恢复。
// 不负责配置写入；配置仍通过 data/fallback.json + 热重载修改。

const GATEWAY_REFRESH_MS = 15000;

const STRATEGY_LABEL = {
  quality_first: '质量优先',
  cost_first: '成本优先',
  free_first: '免费优先',
};

const HEALTH_CLASS = {
  healthy: 'green',
  rate_limited: 'yellow',
  invalid: 'red',
  error: 'red',
  unknown: 'gray',
};

const HEALTH_TEXT = {
  healthy: '健康',
  rate_limited: '限流',
  invalid: '无效',
  error: '异常',
  unknown: '未检测',
};

const formatNumber = (value) => {
  const n = Number(value);
  if (!Number.isFinite(n)) return '-';
  return new Intl.NumberFormat('zh-CN').format(n);
};

const formatTime = (value) => {
  if (!value) return '-';
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return String(value);
  return d.toLocaleString('zh-CN', { hour12: false });
};

// quota cell: used / limit, 0 means unchecked
const QuotaCell = ({ used, limit }) => {
  if (!limit || limit <= 0) {
    return <span className='gw-quota-unchecked'>不限制</span>;
  }
  const pct = (Number(used || 0) / Number(limit)) * 100;
  const tone = pct >= 90 ? 'critical' : pct >= 75 ? 'warning' : 'normal';
  return (
    <span className={`gw-quota ${tone}`}>
      {formatNumber(used)} / {formatNumber(limit)}
      <small>{pct.toFixed(0)}%</small>
    </span>
  );
};

const FallbackRuntimePanel = () => {
  const [loading, setLoading] = useState(false);
  const [lastUpdated, setLastUpdated] = useState(null);
  const [virtualModels, setVirtualModels] = useState([]);
  const [runtimeRows, setRuntimeRows] = useState([]);
  const [actingId, setActingId] = useState('');

  const loadAll = useCallback(async (silent = false) => {
    if (!silent) setLoading(true);
    try {
      const [vmRes, rtRes] = await Promise.all([
        API.get('/api/fallback/virtual-models'),
        API.get('/api/fallback/deployments/runtime-status'),
      ]);
      if (vmRes.data?.success) {
        setVirtualModels(Array.isArray(vmRes.data.data) ? vmRes.data.data : []);
      }
      if (rtRes.data?.success) {
        setRuntimeRows(Array.isArray(rtRes.data.data) ? rtRes.data.data : []);
      }
      setLastUpdated(new Date());
    } catch (e) {
      if (!silent) showError(e.message || '加载三层网关状态失败');
    } finally {
      if (!silent) setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadAll().then();
    const timer = window.setInterval(() => loadAll(true).then(), GATEWAY_REFRESH_MS);
    return () => window.clearInterval(timer);
  }, [loadAll]);

  const runAction = async (deploymentId, action) => {
    setActingId(`${deploymentId}:${action}`);
    try {
      let url = `/api/fallback/deployments/${encodeURIComponent(deploymentId)}`;
      if (action === 'health-check') {
        url += '/health-check';
      } else if (action === 'recover') {
        url += '/recover';
      }
      const res = await API.post(url);
      if (res.data?.success === false) {
        throw new Error(res.data.message || '操作失败');
      }
      showSuccess(action === 'health-check' ? '健康检查完成' : '已恢复部署');
      await loadAll(true);
    } catch (e) {
      showError(e.message || '操作失败');
    } finally {
      setActingId('');
    }
  };

  const vmRows = useMemo(() => {
    return virtualModels.slice().sort((a, b) =>
      String(a.name).localeCompare(String(b.name), 'zh-CN')
    );
  }, [virtualModels]);

  if (loading && runtimeRows.length === 0) {
    return (
      <div className='gw-loading'>
        <Loader active inline='centered' />
      </div>
    );
  }

  return (
    <div className='gateway-runtime-panel'>
      <div className='gw-header'>
        <div>
          <h2>三层网关状态</h2>
          <span>只读展示 cct/high · cct/low · cct/free 虚拟模型与 deployment 运行状态</span>
        </div>
        <div className='gw-header-meta'>
          <span>最后刷新：{lastUpdated ? formatTime(lastUpdated) : '-'}</span>
          <Button basic icon size='small' onClick={() => loadAll()} title='立即刷新'>
            <Icon name='refresh' />
          </Button>
        </div>
      </div>

      <Message info className='gw-notice'>
        <Icon name='info circle' />
        当前为新版三层虚拟模型网关。配置修改请编辑 <code>data/fallback.json</code> 后调用
        <code> POST /api/fallback/config/reload</code> 热重载，本面板仅展示运行状态。
      </Message>

      {/* 1. Virtual Models */}
      <section className='gw-section'>
        <h3>虚拟模型</h3>
        <Table compact celled striped>
          <Table.Header>
            <Table.Row>
              <Table.HeaderCell>虚拟模型</Table.HeaderCell>
              <Table.HeaderCell>策略</Table.HeaderCell>
              <Table.HeaderCell>Pools</Table.HeaderCell>
              <Table.HeaderCell>降级到 low</Table.HeaderCell>
              <Table.HeaderCell>降级到 free</Table.HeaderCell>
              <Table.HeaderCell>状态</Table.HeaderCell>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {vmRows.length === 0 ? (
              <Table.Row>
                <Table.Cell colSpan='6' textAlign='center'>暂无虚拟模型</Table.Cell>
              </Table.Row>
            ) : (
              vmRows.map((vm) => (
                <Table.Row key={vm.name}>
                  <Table.Cell><strong>{vm.name}</strong></Table.Cell>
                  <Table.Cell>{STRATEGY_LABEL[vm.strategy] || vm.strategy || '-'}</Table.Cell>
                  <Table.Cell>{(vm.pools || []).join(', ') || '-'}</Table.Cell>
                  <Table.Cell>{vm.allow_degrade_to_low ? '允许' : '禁止'}</Table.Cell>
                  <Table.Cell>{vm.allow_degrade_to_free ? '允许' : '禁止'}</Table.Cell>
                  <Table.Cell>
                    <Label color={vm.enabled ? 'green' : 'grey'}>
                      {vm.enabled ? 'enabled' : 'disabled'}
                    </Label>
                  </Table.Cell>
                </Table.Row>
              ))
            )}
          </Table.Body>
        </Table>
      </section>

      {/* 2. Runtime Status */}
      <section className='gw-section'>
        <h3>Deployment 运行状态</h3>
        <span className='gw-section-sub'>四维限额预检：RPM / RPD / TPM / TPD 实时用量与上限</span>
        <div className='gw-table-wrap'>
          <Table compact celled striped>
            <Table.Header>
              <Table.Row>
                <Table.HeaderCell>Deployment</Table.HeaderCell>
                <Table.HeaderCell>Pool</Table.HeaderCell>
                <Table.HeaderCell>真实模型</Table.HeaderCell>
                <Table.HeaderCell>RPM</Table.HeaderCell>
                <Table.HeaderCell>RPD</Table.HeaderCell>
                <Table.HeaderCell>TPM</Table.HeaderCell>
                <Table.HeaderCell>TPD</Table.HeaderCell>
                <Table.HeaderCell>健康</Table.HeaderCell>
                <Table.HeaderCell>最近错误</Table.HeaderCell>
                <Table.HeaderCell>操作</Table.HeaderCell>
              </Table.Row>
            </Table.Header>
            <Table.Body>
              {runtimeRows.length === 0 ? (
                <Table.Row>
                  <Table.Cell colSpan='10' textAlign='center'>暂无 deployment 运行数据</Table.Cell>
                </Table.Row>
              ) : (
                runtimeRows.map((row) => {
                  const health = row.health || 'unknown';
                  return (
                    <Table.Row key={row.deployment_id}>
                      <Table.Cell><strong>{row.deployment_id}</strong></Table.Cell>
                      <Table.Cell><code>{row.pool || '-'}</code></Table.Cell>
                      <Table.Cell><span className='gw-code'>{row.real_model || '-'}</span></Table.Cell>
                      <Table.Cell><QuotaCell used={row.minute_requests} limit={row.rpm_limit} /></Table.Cell>
                      <Table.Cell><QuotaCell used={row.day_requests} limit={row.rpd_limit} /></Table.Cell>
                      <Table.Cell><QuotaCell used={row.minute_tokens} limit={row.tpm_limit} /></Table.Cell>
                      <Table.Cell><QuotaCell used={row.day_tokens} limit={row.tpd_limit} /></Table.Cell>
                      <Table.Cell>
                        <span className='gw-health'>
                          <span className={`gw-health-dot ${HEALTH_CLASS[health] || 'gray'}`} />
                          {HEALTH_TEXT[health] || health}
                        </span>
                      </Table.Cell>
                      <Table.Cell className='gw-last-error'>
                        {row.last_error ? (
                          <>
                            <span>{row.last_error}</span>
                            <small>{formatTime(row.last_error_at)}</small>
                          </>
                        ) : '-'}
                      </Table.Cell>
                      <Table.Cell>
                        <Button.Group size='mini'>
                          <Button
                            basic
                            color='blue'
                            loading={actingId === `${row.deployment_id}:health-check`}
                            disabled={Boolean(actingId)}
                            onClick={() => runAction(row.deployment_id, 'health-check')}
                            title='对该 deployment 发起一次健康检查'
                          >
                            <Icon name='heartbeat' /> 检查
                          </Button>
                          <Button
                            basic
                            color='green'
                            loading={actingId === `${row.deployment_id}:recover`}
                            disabled={Boolean(actingId)}
                            onClick={() => runAction(row.deployment_id, 'recover')}
                            title='恢复部署并重置状态'
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
      </section>
    </div>
  );
};

export default FallbackRuntimePanel;
