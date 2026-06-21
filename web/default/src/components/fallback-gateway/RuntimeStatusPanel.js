import React, { useCallback, useEffect, useState } from 'react';
import { Button, Icon, Label, Loader } from 'semantic-ui-react';
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

const progressClass = (pct) => {
  if (pct >= 95) return 'danger';
  if (pct >= 60) return 'warn';
  return 'ok';
};

const QuotaBar = ({ label, used, limit }) => {
  if (!limit || limit <= 0) {
    return (
      <div style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 12 }}>
        <span style={{ width: 32, color: '#9ca3af' }}>{label}</span>
        <span className='gateway-muted'>不限</span>
      </div>
    );
  }
  const pct = Math.min(100, (Number(used || 0) / Number(limit)) * 100);
  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 12 }}>
      <span style={{ width: 32, fontWeight: 500 }}>{label}</span>
      <div className='gateway-progress' style={{ flex: 1 }}>
        <div
          className={`gateway-progress-bar ${progressClass(pct)}`}
          style={{ width: `${pct}%` }}
        />
      </div>
      <span style={{ width: 80, textAlign: 'right', fontSize: 11, color: '#6b7280' }}>
        {formatNumber(used)}/{formatNumber(limit)} ({pct.toFixed(0)}%)
      </span>
    </div>
  );
};

const RuntimeStatusPanel = ({ onReload }) => {
  const [loading, setLoading] = useState(false);
  const [rows, setRows] = useState([]);
  const [lastUpdated, setLastUpdated] = useState(null);
  const [actingAction, setActingAction] = useState('');
  const [expandedRows, setExpandedRows] = useState({});

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

  const toggleRow = (id) =>
    setExpandedRows((prev) => ({ ...prev, [id]: !prev[id] }));

  if (loading && rows.length === 0) {
    return (
      <div style={{ textAlign: 'center', padding: 40 }}>
        <Loader active inline='centered' />
      </div>
    );
  }

  return (
    <div>
      {/* Toolbar */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12, flexWrap: 'wrap', gap: 8 }}>
        <span style={{ fontSize: 12, color: '#9ca3af' }}>
          最后刷新：{lastUpdated ? formatTime(lastUpdated) : '-'}
        </span>
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

      {/* Deployment cards */}
      {rows.length === 0 ? (
        <div style={{ textAlign: 'center', padding: 30, color: '#9ca3af' }}>暂无运行数据</div>
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
          {rows.map((row) => {
            const health = row.health || 'unknown';
            const isExpanded = !!expandedRows[row.deployment_id];
            return (
              <div key={row.deployment_id} className='gateway-section'>
                {/* Summary row — always visible */}
                <div
                  className='gateway-section-header'
                  onClick={() => toggleRow(row.deployment_id)}
                >
                  <div className='gateway-section-header-title'>
                    <Icon name={isExpanded ? 'chevron down' : 'chevron right'} style={{ fontSize: 12 }} />
                    <strong style={{ fontSize: 13 }}>{row.deployment_id}</strong>
                    <Label basic size='mini' color={row.quota_mode === 'free' ? 'green' : 'teal'}>
                      {row.quota_mode || 'normal'}
                    </Label>
                  </div>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                    <span style={{ display: 'inline-flex', alignItems: 'center', gap: 4, fontSize: 12 }}>
                      <span
                        className='dot'
                        style={{
                          display: 'inline-block',
                          width: 8,
                          height: 8,
                          borderRadius: '50%',
                          background: HEALTH_COLOR[health] || '#94a3b8',
                        }}
                      />
                      {HEALTH_TEXT[health] || health}
                    </span>
                    <span style={{ fontSize: 11, color: '#9ca3af' }}>
                      RPM {formatNumber(row.minute_requests)}/{formatNumber(row.rpm_limit || 0)}
                    </span>
                  </div>
                </div>

                {/* Expanded details */}
                {isExpanded && (
                  <div className='gateway-section-body'>
                    {/* Quota progress bars */}
                    <div style={{ display: 'flex', flexDirection: 'column', gap: 6, marginBottom: 12 }}>
                      <QuotaBar label='RPM' used={row.minute_requests} limit={row.rpm_limit} />
                      <QuotaBar label='RPD' used={row.day_requests} limit={row.rpd_limit} />
                      <QuotaBar label='TPM' used={row.minute_tokens} limit={row.tpm_limit} />
                      <QuotaBar label='TPD' used={row.day_tokens} limit={row.tpd_limit} />
                    </div>

                    {/* Additional stats */}
                    <div className='dep-detail-grid'>
                      <div className='dep-detail-item'>
                        <span className='label'>Pool</span>
                        <code>{row.pool || '-'}</code>
                      </div>
                      <div className='dep-detail-item'>
                        <span className='label'>Cooldown</span>
                        {row.cooldown_until ? formatTime(row.cooldown_until) : '-'}
                      </div>
                      <div className='dep-detail-item'>
                        <span className='label'>最近错误</span>
                        <span style={{ fontSize: 11, color: row.last_error ? '#991b1b' : '#9ca3af' }}>
                          {row.last_error || '无'}
                        </span>
                      </div>
                      <div className='dep-detail-item'>
                        <span className='label'>成功率</span>
                        {formatPercent(row.success_rate)}
                      </div>
                      <div className='dep-detail-item'>
                        <span className='label'>平均延迟</span>
                        {row.avg_latency_ms ? `${formatNumber(row.avg_latency_ms)}ms` : '-'}
                      </div>
                    </div>
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
};

export default RuntimeStatusPanel;
