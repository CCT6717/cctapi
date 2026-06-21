import React, { useCallback, useEffect, useState } from 'react';
import { Button, Icon, Loader, Message, Tab } from 'semantic-ui-react';
import { showError, showSuccess } from '../../helpers';
import {
  getGatewayConfig,
  reloadConfig,
  saveGatewayConfig,
  syncFreePool,
  cleanupDryRun,
} from './gatewayConfigApi';
import VirtualModelsEditor from './VirtualModelsEditor';
import DeploymentsEditor from './DeploymentsEditor';
import FreeProvidersEditor from './FreeProvidersEditor';
import ConfigPreview from './ConfigPreview';
import './FallbackGatewayEditor.css';

const VM_LABELS = {
  'cct/high': '高质量模型',
  'cct/low':  '低成本模型',
  'cct/free': '免费模型',
};
const VM_COLORS = {
  'cct/high': '#6366f1',
  'cct/low':  '#06b6d4',
  'cct/free': '#10b981',
};
const POOL_CN = {
  paid_high: '付费高质量池',
  cheap: '低成本池',
  free: '免费池',
  local: '本地池',
};
const STRATEGY_CN = {
  quality_first: '质量优先',
  cost_first: '成本优先',
  free_first: '免费优先',
};

const InlineSummary = ({ config }) => {
  const vms = config?.virtual_models || {};
  const fps = config?.free_providers || {};

  return (
    <div className='gateway-summary-inline'>
      {Object.keys(VM_LABELS).map((key) => {
        const vm = vms[key];
        if (!vm) return null;
        const pools = Array.isArray(vm.pools) ? vm.pools.map((p) => POOL_CN[p] || p).join(', ') : '-';
        const strategy = STRATEGY_CN[vm.strategy] || vm.strategy || '-';
        return (
          <span key={key} className='gateway-summary-item'>
            <span className='label' style={{ color: VM_COLORS[key] }}>{VM_LABELS[key]}:</span>
            {pools} · {strategy}
            <span className={`gateway-badge ${vm.enabled ? 'enabled' : 'disabled'}`} style={{ marginLeft: 4 }}>
              {vm.enabled ? '已启用' : '已停用'}
            </span>
          </span>
        );
      })}
      {Object.entries(fps).map(([k, p]) => (
        <span key={k} className='gateway-summary-item'>
          <span className='label'>{k === 'openrouter' ? 'OpenRouter' : k === 'groq' ? 'Groq' : k}:</span>
          <span className={`gateway-badge ${p.enabled ? 'enabled' : 'disabled'}`}>
            {p.enabled ? `已启用（${p.key_count || 0} 个密钥）` : '已停用'}
          </span>
        </span>
      ))}
    </div>
  );
};

const FallbackGatewayEditor = () => {
  const [config, setConfig] = useState(null);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [actingAction, setActingAction] = useState('');
  const [activeTab, setActiveTab] = useState(0);

  const loadConfig = useCallback(async (silent = false) => {
    if (!silent) setLoading(true);
    try {
      const res = await getGatewayConfig();
      const { success, data, message } = res.data || {};
      if (success !== false && data) {
        setConfig(data);
      } else {
        showError(message || '加载网关配置失败');
      }
    } catch (e) {
      showError(e.message || '加载网关配置失败');
    } finally {
      if (!silent) setLoading(false);
    }
  }, []);

  useEffect(() => { loadConfig().then(); }, [loadConfig]);

  const handleSave = async () => {
    if (!config) return;
    setSaving(true);
    try {
      const res = await saveGatewayConfig(config);
      const { success, data, message } = res.data || {};
      if (success !== false) {
        setConfig(data || config);
        showSuccess('网关配置已保存');
      } else {
        showError(message || '保存网关配置失败');
      }
    } catch (e) {
      showError(e.message || '保存网关配置失败');
    } finally {
      setSaving(false);
    }
  };

  const handleReload = async () => {
    setActingAction('reload');
    try {
      const res = await reloadConfig();
      if (res.data?.success !== false) {
        showSuccess('配置已重新加载');
        await loadConfig(true);
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
      if (res.data?.success !== false) {
        showSuccess('免费池同步完成');
        await loadConfig(true);
      } else {
        showError(res.data?.message || '免费池同步失败');
      }
    } catch (e) {
      showError(e.message || '免费池同步失败');
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
        showSuccess(`清理预检完成：${removed} 项可清理`);
      } else {
        showError(res.data?.message || '清理预检失败');
      }
    } catch (e) {
      showError(e.message || '清理预检失败');
    } finally {
      setActingAction('');
    }
  };

  const updateVM = (v) => setConfig((p) => ({ ...p, virtual_models: v }));
  const updateDeps = (v) => setConfig((p) => ({ ...p, deployments: v }));
  const updateFPs = (v) => setConfig((p) => ({ ...p, free_providers: v }));

  if (loading) {
    return (
      <div style={{ textAlign: 'center', padding: 60 }}>
        <Loader active inline='centered' />
        <p style={{ marginTop: 12, color: '#868b94' }}>加载网关配置中...</p>
      </div>
    );
  }

  if (!config) {
    return (
      <div>
        <Message warning>
          <Icon name='exclamation triangle' />
          未加载到网关配置，请检查后端 API 是否可用。
        </Message>
        <Button onClick={() => loadConfig()} loading={loading}>
          <Icon name='refresh' /> 重新加载
        </Button>
      </div>
    );
  }

  const tabPanes = [
    {
      menuItem: { key: 'vm', content: '虚拟模型' },
      render: () => (
        <Tab.Pane attached={false}>
          <VirtualModelsEditor virtualModels={config.virtual_models || {}} deployments={config.deployments || {}} onChange={updateVM} />
        </Tab.Pane>
      ),
    },
    {
      menuItem: { key: 'dep', content: '模型部署' },
      render: () => (
        <Tab.Pane attached={false}>
          <DeploymentsEditor deployments={config.deployments || {}} onChange={updateDeps} />
        </Tab.Pane>
      ),
    },
    {
      menuItem: { key: 'fp', content: '免费供应商' },
      render: () => (
        <Tab.Pane attached={false}>
          <FreeProvidersEditor freeProviders={config.free_providers || {}} onChange={updateFPs} />
        </Tab.Pane>
      ),
    },
    {
      menuItem: { key: 'preview', content: '配置预览' },
      render: () => (
        <Tab.Pane attached={false}>
          <ConfigPreview config={config} onSave={handleSave} saving={saving} />
        </Tab.Pane>
      ),
    },
  ];

  return (
    <div className='gateway-editor'>
      <div className='gateway-toolbar'>
        <div className='gateway-toolbar-title'>
          <h2>网关编辑器</h2>
          <p>编辑网关配置、模型部署和免费供应商。系统当前使用新版网关结构，旧版路由字段已废弃。</p>
        </div>
        <div className='gateway-toolbar-actions'>
          <Button basic size='small' onClick={handleReload} loading={actingAction === 'reload'} disabled={!!actingAction}>
            <Icon name='sync' /> 重新加载配置
          </Button>
          <Button basic size='small' onClick={handleSyncFreePool} loading={actingAction === 'sync'} disabled={!!actingAction}>
            <Icon name='lightning' /> 同步免费池
          </Button>
          <Button basic size='small' onClick={handleCleanupDryRun} loading={actingAction === 'dryrun'} disabled={!!actingAction}>
            <Icon name='search' /> 清理预检
          </Button>
          <Button className='gateway-btn-primary' size='small' onClick={handleSave} loading={saving} disabled={saving}>
            <Icon name='save' /> 保存网关配置
          </Button>
        </div>
      </div>

      <InlineSummary config={config} />

      <Tab
        menu={{ secondary: true, pointing: true }}
        panes={tabPanes}
        activeIndex={activeTab}
        onTabChange={(_, { activeIndex }) => setActiveTab(activeIndex)}
      />
    </div>
  );
};

export default FallbackGatewayEditor;
