import React, { useCallback, useEffect, useState } from 'react';
import { Button, Icon, Label, Loader, Table } from 'semantic-ui-react';
import { showError, showSuccess } from '../../helpers';
import { cleanupDryRun, getRuntimeStatus, reloadConfig, syncFreePool } from './gatewayConfigApi';

const REFRESH_MS = 15000;

const HEALTH_COLOR = {
  healthy: '#22c55e',
  rate_limited: '#f97316',
  invalid: '#ef4444',
  error: '#ef4444',
  unknown: '#94a3b8',
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

const formatPercent = (value) => {
  if (value === undefined || value === null || value === '') return '-';
  const n = Number(value);
  if (!Number.isFinite(n)) return String(value);
  return `${n.toFixed(1)}%`;
};

const formatTime = (value) => {
  if (!value) return '-';
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return String(value);
  return d.toLocaleString('zh-CN', { hour12: false });
};

const QuotaCell = ({ used, limit }) => {
  if (!limit || limit <= 0) {
    return <span style={{ color: '#94a3b8' }}>不限</span>;
  }
  const pct = (Number(used || 0) / Number(limit)) * 100;
  const color = pct >= 90 ? '#ef4444' : pct >= 75 ? '#f97316' : '#22c55e';
  return (
    <span>
      {formatNumber(used)} / {formatNumber(limit)}
      <small style={{ color, marginLeft: 4 }}>{pct.toFixed(0)}%</small>
    </span>
  );
};

const RuntimeStatusPanel = ({ onReload }) => {
  const [loading, setLoading] = useState(false);
  const [rows, setRows] = useState([]);
  const [lastUpdated, setLastUpdated] = useState(null);
  const [actingAction, setActingAction] = useState('');

  const loadStatus = useCallback(async (silent = false) => {
    if (!silent) setLoading(true);
    try {
      const res = await getRuntimeStatus();
      const { success, data } = res.data || {};
      if (success) {
        setRows(Array.isArray(data) ? data : []);
      }
      setLastUpdated(new Date());
    } catch (e) {
      if (!silent) showError(e.message || '加载运行状态失败');
    } finally {
      if (!silent) setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadStatus().then();
    const timer = window.setInterval(() => loadStatus(true).then(), REFRESH_MS);
    return () => window.clearInterval(timer);
  }, [loadStatus]);

  const handleReload = async () => {
    setActingAction('reload');
    try {
      const res = await reloadConfig();
      if (res.data?.success) {
        showSuccess('配置已重新加载');
        if (onReload) onReload();
      } else {
        showError(res.data?.message || '重新加载配置失败');
      }
    } catch (e) {
      showError(e.message || '重新加载配置失败');
    } finally {
      setActingAction('');
    }
  };

  const handleSyncFreePool = async () => {
    setActingAction('sync');
    try {
      const res = await syncFreePool();
      if (res.data?.success) {
        showSuccess('Free Pool 同步完成');
        if (onReload) onReload();
      } else {
        showError(res.data?.message || 'Free Pool 同步失败');
      }
    } catch (e) {
      showError(e.message || 'Free Pool 同步失败');
    } finally {
      setActingAction('');
    }
  };

  const handleCleanupDryRun = async () => {
    setActingAction('dryrun');
    try {
      const res = await cleanupDryRun();
      if (res.data?.success) {
        const result = res.data.data || res.data.result || {};
        const removed = Array.isArray(result.removed) ? result.removed.length : (result.removed_count || 0);
        showSuccess(`Dry Run 完成：${removed} 个将被清理`);
      } else {
        showError(res.data?.message || 'Cleanup Dry Run 失败');
      }
    } catch (e) {
      showError(e.message || 'Cleanup Dry Run 失败');
    } finally {
      setActingAction('');
    }
  };

  if (loading && rows.length === 0) {
    return (
      <div style={{ textAlign: 'center', padding: 40 }}>
        <Loader active inline='centered' />
      </div>
    );
  }

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12 }}>
        <div>
          <span>最后刷新：{lastUpdated ? formatTime(lastUpdated) : '-'}</span>
        </div>
        <div style={{ display: 'flex', gap: 6 }}>
          <Button basic size='small' onClick={() => loadStatus()} loading={loading}>
            <Icon name='refresh' /> 刷新
          </Button>
          <Button basic size='small' onClick={handleReload} loading={actingAction === 'reload'}>
            <Icon name='sync' /> 重载配置
          </Button>
          <Button basic size='small' onClick={handleSyncFreePool} loading={actingAction === 'sync'}>
            <Icon name='lightning' /> Free Pool 同步
          </Button>
          <Button basic size='small' onClick={handleCleanupDryRun} loading={actingAction === 'dryrun'}>
            <Icon name='search' /> Cleanup Dry Run
          </Button>
        </div>
      </div>

      <div style={{ overflowX: 'auto' }}>
        <Table compact celled striped size='small'>
          <Table.Header>
            <Table.Row>
              <Table.HeaderCell>Deployment</Table.HeaderCell>
              <Table.HeaderCell>Pool</Table.HeaderCell>
              <Table.HeaderCell>状态</Table.HeaderCell>
              <Table.HeaderCell>Quota Mode</Table.HeaderCell>
              <Table.HeaderCell>RPM</Table.HeaderCell>
              <Table.HeaderCell>RPD</Table.HeaderCell>
              <Table.HeaderCell>TPM</Table.HeaderCell>
              <Table.HeaderCell>TPD</Table.HeaderCell>
              <Table.HeaderCell>Cooldown</Table.HeaderCell>
              <Table.HeaderCell>最近错误</Table.HeaderCell>
              <Table.HeaderCell>成功率</Table.HeaderCell>
              <Table.HeaderCell>平均延迟</Table.HeaderCell>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {rows.length === 0 ? (
              <Table.Row>
                <Table.Cell colSpan='12' textAlign='center'>暂无运行数据</Table.Cell>
              </Table.Row>
            ) : (
              rows.map((row) => {
                const health = row.health || 'unknown';
                return (
                  <Table.Row key={row.deployment_id}>
                    <Table.Cell><strong>{row.deployment_id}</strong></Table.Cell>
                    <Table.Cell><code>{row.pool || '-'}</code></Table.Cell>
                    <Table.Cell>
                      <span className='gw-health'>
                        <span className={`gw-health-dot ${HEALTH_CLASS[health] || 'gray'}`} />
                        {HEALTH_TEXT[health] || health}
                      </span>
                    </Table.Cell>
                    <Table.Cell>
                      <Label basic size='mini' color={row.quota_mode === 'free' ? 'blue' : 'teal'}>
                        {row.quota_mode || 'normal'}
                      </Label>
                    </Table.Cell>
                    <Table.Cell><QuotaCell used={row.minute_requests} limit={row.rpm_limit} /></Table.Cell>
                    <Table.Cell><QuotaCell used={row.day_requests} limit={row.rpd_limit} /></Table.Cell>
                    <Table.Cell><QuotaCell used={row.minute_tokens} limit={row.tpm_limit} /></Table.Cell>
                    <Table.Cell><QuotaCell used={row.day_tokens} limit={row.tpd_limit} /></Table.Cell>
                    <Table.Cell>
                      {row.cooldown_until ? formatTime(row.cooldown_until) : '-'}
                    </Table.Cell>
                    <Table.Cell>
                      {row.last_error ? (
                        <span style={{ fontSize: 12 }}>{row.last_error}</span>
                      ) : '-'}
                    </Table.Cell>
                    <Table.Cell>{formatPercent(row.success_rate)}</Table.Cell>
                    <Table.Cell>
                      {row.avg_latency_ms ? `${formatNumber(row.avg_latency_ms)}ms` : '-'}
                    </Table.Cell>
                  </Table.Row>
                );
              })
            )}
          </Table.Body>
        </Table>
      </div>
    </div>
  );
};

export default RuntimeStatusPanel;
