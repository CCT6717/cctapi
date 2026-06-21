import React, { useCallback, useEffect, useState } from 'react';
import { Button, Icon, Loader, Message, Tab } from 'semantic-ui-react';
import { showError, showSuccess } from '../../helpers';
import {
  getGatewayConfig,
  reloadConfig,
  saveGatewayConfig,
  syncFreePool,
} from './gatewayConfigApi';
import VirtualModelsEditor from './VirtualModelsEditor';
import DeploymentsEditor from './DeploymentsEditor';
import FreeProvidersEditor from './FreeProvidersEditor';
import RuntimeStatusPanel from './RuntimeStatusPanel';
import ConfigPreview from './ConfigPreview';

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

  useEffect(() => {
    loadConfig().then();
  }, [loadConfig]);

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
        showSuccess('Free Pool 同步完成');
        await loadConfig(true);
      } else {
        showError(res.data?.message || 'Free Pool 同步失败');
      }
    } catch (e) {
      showError(e.message || 'Free Pool 同步失败');
    } finally {
      setActingAction('');
    }
  };

  const updateVirtualModels = (updatedVMs) => {
    setConfig((prev) => ({ ...prev, virtual_models: updatedVMs }));
  };

  const updateDeployments = (updatedDeps) => {
    setConfig((prev) => ({ ...prev, deployments: updatedDeps }));
  };

  const updateFreeProviders = (updatedFPs) => {
    setConfig((prev) => ({ ...prev, free_providers: updatedFPs }));
  };

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
          未加载到网关配置。请检查后端 API 是否可用。
        </Message>
        <Button onClick={() => loadConfig()} loading={loading}>
          <Icon name='refresh' /> 重新加载
        </Button>
      </div>
    );
  }

  const tabPanes = [
    {
      menuItem: { key: 'vm', icon: 'server', content: '虚拟模型' },
      render: () => (
        <Tab.Pane attached={false}>
          <VirtualModelsEditor
            virtualModels={config.virtual_models || {}}
            onChange={updateVirtualModels}
          />
        </Tab.Pane>
      ),
    },
    {
      menuItem: { key: 'dep', icon: 'cubes', content: '部署' },
      render: () => (
        <Tab.Pane attached={false}>
          <DeploymentsEditor
            deployments={config.deployments || {}}
            onChange={updateDeployments}
          />
        </Tab.Pane>
      ),
    },
    {
      menuItem: { key: 'fp', icon: 'key', content: 'Free Providers' },
      render: () => (
        <Tab.Pane attached={false}>
          <FreeProvidersEditor
            freeProviders={config.free_providers || {}}
            onChange={updateFreeProviders}
          />
        </Tab.Pane>
      ),
    },
    {
      menuItem: { key: 'rt', icon: 'heartbeat', content: '运行状态' },
      render: () => (
        <Tab.Pane attached={false}>
          <RuntimeStatusPanel onReload={() => loadConfig(true)} />
        </Tab.Pane>
      ),
    },
    {
      menuItem: { key: 'preview', icon: 'code', content: '配置预览' },
      render: () => (
        <Tab.Pane attached={false}>
          <ConfigPreview
            config={config}
            onSave={handleSave}
            saving={saving}
          />
        </Tab.Pane>
      ),
    },
  ];

  return (
    <div>
      <div style={{
        display: 'flex',
        justifyContent: 'space-between',
        alignItems: 'center',
        marginBottom: 16,
        flexWrap: 'wrap',
        gap: 8,
      }}>
        <div>
          <h2 style={{ margin: 0 }}>三层网关配置编辑器</h2>
          <span style={{ color: '#868b94', fontSize: 13 }}>
            管理虚拟模型、部署和 Free Providers 的新版网关配置
          </span>
        </div>
        <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
          <Button
            primary
            icon
            labelPosition='left'
            onClick={handleSave}
            loading={saving}
            disabled={saving}
          >
            <Icon name='save' /> 保存
          </Button>
          <Button
            basic
            icon
            labelPosition='left'
            onClick={handleReload}
            loading={actingAction === 'reload'}
            disabled={!!actingAction}
          >
            <Icon name='sync' /> 重载配置
          </Button>
          <Button
            basic
            icon
            labelPosition='left'
            onClick={handleSyncFreePool}
            loading={actingAction === 'sync'}
            disabled={!!actingAction}
          >
            <Icon name='lightning' /> Free Pool 同步
          </Button>
        </div>
      </div>

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
