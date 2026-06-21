import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { Button, Header, Icon, Label, Loader, Message, Table } from 'semantic-ui-react';
import { showError, showSuccess } from '../../helpers';
import {
  cleanupDryRun,
  getGatewayConfig,
  getRuntimeStatus,
  reloadConfig,
  saveGatewayConfig,
  syncFreePool,
} from './gatewayConfigApi';
import FreeProvidersEditor from './FreeProvidersEditor';
import {
  isFreeDeployment,
  providerFromDeploymentId,
} from './freePoolUtils';

const POOL_LABELS = {
  free: '免费池',
};

const STRATEGY_LABELS = {
  free_first: '免费优先',
};

const PROVIDER_LABELS = {
  openrouter: 'OpenRouter',
  groq: 'Groq',
};

const formatNumber = (value) => {
  const n = Number(value);
  if (!Number.isFinite(n)) return '-';
  return new Intl.NumberFormat('zh-CN').format(n);
};

const formatLimit = (value) => {
  if (value === undefined || value === null || value === '') return '默认';
  const n = Number(value);
  if (!Number.isFinite(n)) return '-';
  return n === 0 ? '不限' : formatNumber(n);
};

const FreeModelPool = () => {
  const [config, setConfig] = useState(null);
  const [runtimeRows, setRuntimeRows] = useState([]);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [actingAction, setActingAction] = useState('');

  const loadAll = useCallback(async (silent = false) => {
    if (!silent) setLoading(true);
    try {
      const [configRes, runtimeRes] = await Promise.all([
        getGatewayConfig(),
        getRuntimeStatus(),
      ]);
      const configData = configRes.data || {};
      if (configData.success !== false && configData.data) {
        setConfig(configData.data);
      } else {
        showError(configData.message || '加载免费模型池失败');
      }
      const runtimeData = runtimeRes.data || {};
      if (runtimeData.success !== false) {
        setRuntimeRows(Array.isArray(runtimeData.data) ? runtimeData.data : []);
      }
    } catch (e) {
      showError(e.message || '加载免费模型池失败');
    } finally {
      if (!silent) setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadAll().then();
  }, [loadAll]);

  const updateFreeProviders = (updatedFreeProviders) => {
    setConfig((prev) => ({ ...prev, free_providers: updatedFreeProviders }));
  };

  const saveFreePoolConfig = async () => {
    if (!config) return;
    setSaving(true);
    try {
      const res = await saveGatewayConfig(config);
      const { success, data, message } = res.data || {};
      if (success !== false) {
        setConfig(data || config);
        showSuccess('免费模型池配置已保存');
      } else {
        showError(message || '保存免费模型池失败');
      }
    } catch (e) {
      showError(e.message || '保存免费模型池失败');
    } finally {
      setSaving(false);
    }
  };

  const runAction = async (action, fn, successMessage) => {
    setActingAction(action);
    try {
      const res = await fn();
      if (res.data?.success !== false) {
        showSuccess(successMessage);
        await loadAll(true);
      } else {
        showError(res.data?.message || `${successMessage}失败`);
      }
    } catch (e) {
      showError(e.message || `${successMessage}失败`);
    } finally {
      setActingAction('');
    }
  };

  const runCleanupDryRun = async () => {
    setActingAction('dryrun');
    try {
      const res = await cleanupDryRun();
      if (res.data?.success !== false) {
        const result = res.data.data || res.data.result || {};
        const staleChannels = Array.isArray(result.stale_channels)
          ? result.stale_channels.length
          : 0;
        const staleDeployments = Array.isArray(result.stale_deployments)
          ? result.stale_deployments.length
          : 0;
        showSuccess(`清理预检完成：${staleChannels} 个渠道，${staleDeployments} 个部署`);
      } else {
        showError(res.data?.message || '清理预检失败');
      }
    } catch (e) {
      showError(e.message || '清理预检失败');
    } finally {
      setActingAction('');
    }
  };

  const freeModel = config?.virtual_models?.['cct/free'];
  const freeDeployments = useMemo(() => {
    const deployments = config?.deployments || {};
    return Object.keys(deployments)
      .filter((id) => isFreeDeployment(id, deployments[id]))
      .sort()
      .map((id) => {
        const runtime = runtimeRows.find((row) => row.deployment_id === id) || {};
        return { id, ...deployments[id], runtime };
      });
  }, [config, runtimeRows]);

  const enabledFreeCount = freeDeployments.filter((dep) => dep.enabled !== false).length;

  if (loading) {
    return (
      <div style={{ textAlign: 'center', padding: 60 }}>
        <Loader active inline='centered' />
        <p style={{ marginTop: 12, color: '#868b94' }}>加载免费模型池中...</p>
      </div>
    );
  }

  if (!config) {
    return <Message warning>未加载到免费模型池配置。</Message>;
  }

  return (
    <div className='fallback-config-panel'>
      <div className='fallback-config-toolbar'>
        <Header as='h2' className='fallback-config-title'>
          免费模型池
          <Header.Subheader>
            管理免费模型、免费供应商、限额覆盖和自动生成的免费部署。
          </Header.Subheader>
        </Header>
        <div className='fallback-config-actions'>
          <Button basic icon labelPosition='left' onClick={() => loadAll()} loading={loading}>
            <Icon name='refresh' /> 刷新状态
          </Button>
          <Button
            basic
            icon
            labelPosition='left'
            onClick={() => runAction('reload', reloadConfig, '配置已重新加载')}
            loading={actingAction === 'reload'}
            disabled={!!actingAction}
          >
            <Icon name='sync' /> 重新加载配置
          </Button>
          <Button
            basic
            icon
            labelPosition='left'
            onClick={() => runAction('sync', syncFreePool, '免费池同步完成')}
            loading={actingAction === 'sync'}
            disabled={!!actingAction}
          >
            <Icon name='lightning' /> 同步免费池
          </Button>
          <Button
            basic
            icon
            labelPosition='left'
            onClick={runCleanupDryRun}
            loading={actingAction === 'dryrun'}
            disabled={!!actingAction}
          >
            <Icon name='search' /> 清理预检
          </Button>
          <Button
            primary
            icon
            labelPosition='left'
            onClick={saveFreePoolConfig}
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
            <h3>免费模型总览</h3>
            <span>cct/free 使用免费池和免费优先策略。</span>
          </div>
        </div>
        <Table compact celled striped>
          <Table.Header>
            <Table.Row>
              <Table.HeaderCell>免费模型</Table.HeaderCell>
              <Table.HeaderCell>路由池</Table.HeaderCell>
              <Table.HeaderCell>路由策略</Table.HeaderCell>
              <Table.HeaderCell>状态</Table.HeaderCell>
              <Table.HeaderCell>可用免费部署</Table.HeaderCell>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            <Table.Row>
              <Table.Cell>
                <strong>免费模型</strong>
                <div><Label basic color='green'>cct/free</Label></div>
              </Table.Cell>
              <Table.Cell>
                {(freeModel?.pools || ['free'])
                  .map((pool) => POOL_LABELS[pool] || pool)
                  .join(' / ')}
              </Table.Cell>
              <Table.Cell>{STRATEGY_LABELS[freeModel?.strategy] || freeModel?.strategy || '免费优先'}</Table.Cell>
              <Table.Cell>
                {freeModel?.enabled === false ? (
                  <Label basic color='grey'>已停用</Label>
                ) : (
                  <Label basic color='green'>已启用</Label>
                )}
              </Table.Cell>
              <Table.Cell>{enabledFreeCount} / {freeDeployments.length}</Table.Cell>
            </Table.Row>
          </Table.Body>
        </Table>
      </section>

      <section className='fallback-virtual-panel'>
        <div className='fallback-virtual-header'>
          <div>
            <h3>免费供应商</h3>
            <span>OpenRouter、Groq 和限额覆盖。不会显示完整 API key。</span>
          </div>
        </div>
        <FreeProvidersEditor
          freeProviders={config.free_providers || {}}
          onChange={updateFreeProviders}
        />
      </section>

      <section className='fallback-virtual-panel'>
        <div className='fallback-virtual-header'>
          <div>
            <h3>免费模型部署</h3>
            <span>自动生成的免费部署只读展示，请在免费供应商区域管理。</span>
          </div>
        </div>
        <Table compact celled striped>
          <Table.Header>
            <Table.Row>
              <Table.HeaderCell>部署 ID</Table.HeaderCell>
              <Table.HeaderCell>供应商</Table.HeaderCell>
              <Table.HeaderCell>真实模型</Table.HeaderCell>
              <Table.HeaderCell>额度模式</Table.HeaderCell>
              <Table.HeaderCell>RPM</Table.HeaderCell>
              <Table.HeaderCell>RPD</Table.HeaderCell>
              <Table.HeaderCell>TPM</Table.HeaderCell>
              <Table.HeaderCell>TPD</Table.HeaderCell>
              <Table.HeaderCell>状态</Table.HeaderCell>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {freeDeployments.length === 0 ? (
              <Table.Row>
                <Table.Cell colSpan='9' textAlign='center'>暂无免费模型部署</Table.Cell>
              </Table.Row>
            ) : (
              freeDeployments.map((dep) => {
                const provider = providerFromDeploymentId(dep.id);
                return (
                  <Table.Row key={dep.id}>
                    <Table.Cell>
                      <strong>{dep.id}</strong>
                      <div style={{ marginTop: 4 }}>
                        <Label basic color='blue' size='mini'>
                          锁定：由免费供应商自动生成
                        </Label>
                      </div>
                    </Table.Cell>
                    <Table.Cell>{PROVIDER_LABELS[provider] || provider}</Table.Cell>
                    <Table.Cell>{dep.real_model || '-'}</Table.Cell>
                    <Table.Cell>{dep.quota_mode || 'free'}</Table.Cell>
                    <Table.Cell>{formatLimit(dep.rpm_limit)}</Table.Cell>
                    <Table.Cell>{formatLimit(dep.rpd_limit)}</Table.Cell>
                    <Table.Cell>{formatLimit(dep.tpm_limit)}</Table.Cell>
                    <Table.Cell>{formatLimit(dep.tpd_limit)}</Table.Cell>
                    <Table.Cell>
                      {dep.enabled === false ? (
                        <Label basic color='grey'>已停用</Label>
                      ) : (
                        <Label basic color={dep.runtime?.health === 'invalid' ? 'red' : 'green'}>
                          {dep.runtime?.health || '已启用'}
                        </Label>
                      )}
                    </Table.Cell>
                  </Table.Row>
                );
              })
            )}
          </Table.Body>
        </Table>
      </section>
    </div>
  );
};

export default FreeModelPool;
