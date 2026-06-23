import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { Loader } from 'semantic-ui-react';
import { API, showError, showSuccess } from '../../helpers';
import './GatewayStatus.css';

const GATEWAY_REFRESH_MS = 15000;

const STRATEGY_LABEL = {
  quality_first: '质量优先',
  cost_first: '成本优先',
  free_first: '免费优先',
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

/* ---- Chevron 箭头 ---- */
function Chevron({ open }) {
  return (
    <svg
      className={`gw-chev${open ? ' open' : ''}`}
      viewBox='0 0 24 24'
      fill='none'
      stroke='currentColor'
      strokeWidth='2.4'
      strokeLinecap='round'
      strokeLinejoin='round'
    >
      <path d='M9 18l6-6-6-6' />
    </svg>
  );
}

/* ---- 配额格子 ---- */
function QuotaCell({ used, limit }) {
  if (!limit || limit <= 0) {
    return (
      <div className='gw-qbox'>
        <div className='gw-qlabel'>限额</div>
        <div className='gw-qval unlimited'>不限制</div>
      </div>
    );
  }
  const pct = (Number(used || 0) / Number(limit)) * 100;
  const tone = pct >= 90 ? 'critical' : pct >= 75 ? 'warning' : '';
  return (
    <div className={`gw-qbox ${tone}`}>
      <div className='gw-qlabel'>限额</div>
      <div className={`gw-qval${tone ? '' : ' ok'}`}>
        {formatNumber(used)} / {formatNumber(limit)}
      </div>
    </div>
  );
}

/* ---- Deployment 子卡片 ---- */
function DeploymentCard({ dep, actingId, onAction }) {
  const health = dep.health || 'unknown';
  return (
    <div className='gw-dep-card'>
      <div className='gw-dep-name'>
        {dep.deployment_id}
        {dep.pool && <span className='gw-tag'>pool · {dep.pool}</span>}
        {dep.real_model && <span className='gw-tag'>{dep.real_model}</span>}
        <span className={`gw-badge${health === 'healthy' ? '' : ' off'}`}>
          <span className='gw-dot' />
          {HEALTH_TEXT[health] || health}
        </span>
      </div>
      <div className='gw-quota-grid'>
        <QuotaCell used={dep.minute_requests} limit={dep.rpm_limit} />
        <QuotaCell used={dep.day_requests} limit={dep.rpd_limit} />
        <QuotaCell used={dep.minute_tokens} limit={dep.tpm_limit} />
        <QuotaCell used={dep.day_tokens} limit={dep.tpd_limit} />
      </div>
      {dep.last_error && (
        <div className='gw-error'>
          {dep.last_error}
          <span className='gw-error-time'>{formatTime(dep.last_error_at)}</span>
        </div>
      )}
      <div className='gw-actions'>
        <button
          className='gw-mini'
          disabled={Boolean(actingId)}
          onClick={() => onAction(dep.deployment_id, 'health-check')}
        >
          检查
        </button>
        <button
          className='gw-mini'
          disabled={Boolean(actingId)}
          onClick={() => onAction(dep.deployment_id, 'recover')}
        >
          恢复
        </button>
      </div>
    </div>
  );
}

/* ---- 虚拟模型行 ---- */
function VirtualModelRow({ vm, actingId, onAction }) {
  const [open, setOpen] = useState(false);
  const canOpen = vm.deployments && vm.deployments.length > 0;

  return (
    <div className='gw-row'>
      <div className='gw-row-top' onClick={() => canOpen && setOpen(!open)}>
        <Chevron open={open} />
        <div>
          <div className='gw-name'>
            {vm.name}
            <span className='gw-tag'>
              {STRATEGY_LABEL[vm.strategy] || vm.strategy || '-'}
            </span>
          </div>
          <div className='gw-meta'>
            策略：{STRATEGY_LABEL[vm.strategy] || vm.strategy || '-'}
            {vm.pools?.length > 0 && ` · pool: ${vm.pools.join(', ')}`}
            {` · ${vm.deployments?.length || 0} 个真实部署`}
          </div>
        </div>
        <div className='gw-spacer' />
        <span className={`gw-badge${vm.enabled ? '' : ' off'}`}>
          <span className='gw-dot' />
          {vm.enabled ? 'enabled' : 'disabled'}
        </span>
      </div>

      <div className='gw-pools'>
        {vm.pools?.map((pool) => (
          <span className='gw-pill' key={pool}>
            pool · {pool}
          </span>
        ))}
        <span className={`gw-pill ${vm.allow_degrade_to_low ? 'on' : 'off'}`}>
          降级 low · {vm.allow_degrade_to_low ? '允许' : '禁止'}
        </span>
        <span className={`gw-pill ${vm.allow_degrade_to_free ? 'on' : 'off'}`}>
          降级 free · {vm.allow_degrade_to_free ? '允许' : '禁止'}
        </span>
      </div>

      {open && (
        <div className='gw-dep'>
          {vm.deployments.map((dep) => (
            <DeploymentCard
              key={dep.deployment_id}
              dep={dep}
              actingId={actingId}
              onAction={onAction}
            />
          ))}
        </div>
      )}
    </div>
  );
}

/* ---- 主组件 ---- */
const GatewayStatus = () => {
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
    return virtualModels
      .filter((vm) => (vm.pools || []).some((pool) => pool === 'free'))
      .sort((a, b) => String(a.name).localeCompare(String(b.name), 'zh-CN'))
      .map((vm) => ({
        ...vm,
        deployments: runtimeRows
          .filter((row) => (vm.pools || []).includes(row.pool))
          .sort((a, b) =>
            String(a.deployment_id).localeCompare(String(b.deployment_id), 'zh-CN')
          ),
      }));
  }, [virtualModels, runtimeRows]);

  if (loading && runtimeRows.length === 0) {
    return (
      <div className='gw-status'>
        <div className='gw-loading'>
          <Loader active inline='centered' />
        </div>
      </div>
    );
  }

  return (
    <div className='gw-status'>
      <div className='gw-panel'>
        <div className='gw-panel-head'>
          <div className='gw-panel-title'>三层网关状态</div>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <span style={{ fontSize: 12, color: 'var(--gw-t3)' }}>
              {lastUpdated ? formatTime(lastUpdated) : '-'}
            </span>
            <button className='gw-refresh' title='刷新' onClick={() => loadAll()}>
              &#x27F3;
            </button>
          </div>
        </div>
        <div className='gw-hint'>
          新版三层虚拟模型网关。配置修改请编辑 <span className='gw-code'>data/fallback.json</span>{' '}
          后调用 <span className='gw-code'>POST /api/fallback/config/reload</span> 热重载，本面板仅展示运行状态。
        </div>
        <div className='gw-list'>
          {vmRows.length === 0 ? (
            <div style={{ textAlign: 'center', padding: 24, color: 'var(--gw-t3)' }}>
              暂无虚拟模型数据
            </div>
          ) : (
            vmRows.map((vm) => (
              <VirtualModelRow
                key={vm.name}
                vm={vm}
                actingId={actingId}
                onAction={runAction}
              />
            ))
          )}
        </div>
      </div>
    </div>
  );
};

export default GatewayStatus;
