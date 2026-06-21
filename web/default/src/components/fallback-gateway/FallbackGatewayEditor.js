import React, { useCallback, useEffect, useState } from 'react';
import { Button, Header, Icon, Loader, Message } from 'semantic-ui-react';
import { showError, showSuccess } from '../../helpers';
import {
  getGatewayConfig,
  reloadConfig,
  saveGatewayConfig,
} from './gatewayConfigApi';
import VirtualModelsEditor from './VirtualModelsEditor';
import DeploymentsEditor from './DeploymentsEditor';

const FallbackGatewayEditor = () => {
  const [config, setConfig] = useState(null);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [actingAction, setActingAction] = useState('');

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

  const updateVirtualModels = (updatedVMs) => {
    setConfig((prev) => ({ ...prev, virtual_models: updatedVMs }));
  };

  const updateDeployments = (updatedDeps) => {
    setConfig((prev) => ({ ...prev, deployments: updatedDeps }));
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

  return (
    <div className='fallback-config-panel'>
      <div className='fallback-config-toolbar'>
        <Header as='h2' className='fallback-config-title'>
          模型编辑器
          <Header.Subheader>
            管理高质量模型、低成本模型和普通模型部署。免费模型请前往「免费模型池」模块管理。
          </Header.Subheader>
        </Header>
        <div className='fallback-config-actions'>
          <Button
            basic
            icon
            labelPosition='left'
            onClick={handleReload}
            loading={actingAction === 'reload'}
            disabled={!!actingAction}
          >
            <Icon name='sync' /> 重新加载配置
          </Button>
          <Button
            primary
            icon
            labelPosition='left'
            onClick={handleSave}
            loading={saving}
            disabled={saving}
          >
            <Icon name='save' /> 保存配置
          </Button>
        </div>
      </div>

      <section className='fallback-virtual-panel'>
        <div className='fallback-virtual-header'>
          <div>
            <h3>虚拟模型</h3>
            <span>只管理高质量模型和低成本模型。路由池与策略按当前配置只读展示。</span>
          </div>
        </div>
        <VirtualModelsEditor
          virtualModels={config.virtual_models || {}}
          onChange={updateVirtualModels}
        />
      </section>

      <section className='fallback-virtual-panel'>
        <div className='fallback-virtual-header'>
          <div>
            <h3>模型部署</h3>
            <span>编辑普通模型部署的真实模型、通道、额度和能力字段。</span>
          </div>
        </div>
        <DeploymentsEditor
          deployments={config.deployments || {}}
          onChange={updateDeployments}
        />
      </section>
    </div>
  );
};

export default FallbackGatewayEditor;
