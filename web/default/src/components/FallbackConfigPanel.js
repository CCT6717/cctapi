import React, { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Button,
  Divider,
  Header,
  Icon,
  Label,
  Loader,
  Message,
  Table,
} from 'semantic-ui-react';
import { API } from '../helpers';
import './FallbackConfigPanel.css';

const ROUTING_MODE_WEIGHTED = 'weighted';
const ROUTING_MODE_SEQUENTIAL = 'sequential';
const ROUTING_MODE_FIXED = 'fixed';
const ROUTING_MODE_META = {
  [ROUTING_MODE_WEIGHTED]: {
    title: '按权重',
    detail: '健康 deployment 按 weight 比例分流，适合主力和备用同时消耗',
    icon: 'random',
    color: 'blue',
  },
  [ROUTING_MODE_SEQUENTIAL]: {
    title: '按顺序',
    detail: '严格按列表顺序尝试，前一个不可用或额度到线后再切下一个',
    icon: 'sort amount down',
    color: 'teal',
  },
  [ROUTING_MODE_FIXED]: {
    title: '固定模型',
    detail: '始终路由到指定真实模型，适合手动锁定主力部署',
    icon: 'bullseye',
    color: 'purple',
  },
};

const STRATEGY_TO_ROUTING_MODE = {
  quality_first: ROUTING_MODE_SEQUENTIAL,
  cost_first: ROUTING_MODE_SEQUENTIAL,
  free_first: ROUTING_MODE_SEQUENTIAL,
  weighted: ROUTING_MODE_WEIGHTED,
  sequential: ROUTING_MODE_SEQUENTIAL,
  fixed: ROUTING_MODE_FIXED,
};

const isSeparatorKey = (id) => String(id || '').startsWith('---');

const getDeploymentOwnerNames = (projectedVMs, deploymentId) =>
  (projectedVMs || [])
    .filter((vm) => (vm.fallback_order || []).includes(deploymentId))
    .map((vm) => vm.name || '未命名虚拟模型');

const formatStatusTime = (value) => {
  if (!value) return '-';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return String(value);
  return date.toLocaleString('zh-CN', { hour12: false });
};

const getDeploymentStatusMeta = (status) => {
  const alertType = status?.alert_type || '';
  if (alertType === 'cooldown') {
    return {
      label: '冷却中',
      color: 'orange',
      detail: `冷却至 ${formatStatusTime(status.cooldown_until)}`,
    };
  }
  if (alertType === 'exhausted') {
    return {
      label: '已耗尽',
      color: 'red',
      detail: `耗尽至 ${formatStatusTime(status.exhausted_until)}`,
    };
  }
  if (alertType === 'hard_limit') {
    return {
      label: '硬限额',
      color: 'red',
      detail: `用量 ${status?.usage_percent || '-'}`,
    };
  }
  if (alertType === 'soft_limit') {
    return {
      label: '软限额',
      color: 'yellow',
      detail: `用量 ${status?.usage_percent || '-'}`,
    };
  }
  return {
    label: '可用',
    color: 'green',
    detail: status ? `用量 ${status.usage_percent || '-'}` : '暂无状态数据',
  };
};

const ModelEditor = ({ highlightDeployment }) => {
  const [config, setConfig] = useState(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [expandedVirtualModels, setExpandedVirtualModels] = useState({});
  const [expandedDeployments, setExpandedDeployments] = useState({});
  const [deploymentStatuses, setDeploymentStatuses] = useState({});

  const VISIBLE_VMS = ['cct/high', 'cct/low'];
  const isFreeDeployment = (id) => String(id || '').startsWith('free:');

  const visibleDeploymentIds = useMemo(() => {
    if (!config?.deployments) return [];
    return Object.keys(config.deployments).filter(
      (id) => {
        if (isSeparatorKey(id)) return false;
        const dep = config.deployments[id];
        if (dep?.pool === 'free' || isFreeDeployment(id)) return false;
        return true;
      }
    );
  }, [config]);

  const vmArray = useMemo(() => {
    if (!config?.virtual_models) return [];
    return VISIBLE_VMS.map((name) => {
      const vm = config.virtual_models[name];
      if (!vm) return null;
      // v2 → v1 projection: derive fallback_order from pools
      const pools = Array.isArray(vm.pools) ? vm.pools : [];
      const fallbackOrder = visibleDeploymentIds.filter((id) => {
        const dep = config.deployments[id];
        return dep && pools.includes(dep.pool);
      });
      // v2 → v1 projection: map strategy to routing_mode
      const routingMode = STRATEGY_TO_ROUTING_MODE[vm.strategy] || vm.routing_mode || ROUTING_MODE_WEIGHTED;
      return {
        name,
        ...vm,
        fallback_order: fallbackOrder,
        routing_mode: routingMode,
      };
    }).filter(Boolean);
  }, [config, visibleDeploymentIds]);

  const deploymentArray = useMemo(() => {
    if (!config?.deployments) return [];
    return visibleDeploymentIds.map((id) => ({
      id,
      ...config.deployments[id],
    }));
  }, [config, visibleDeploymentIds]);

  const deploymentsById = useMemo(() => {
    const map = {};
    deploymentArray.forEach((dep) => {
      map[dep.id] = dep;
    });
    return map;
  }, [deploymentArray]);

  const loadConfig = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const res = await API.get('/api/fallback/gateway/config');
      const { success, data, message } = res.data || {};
      if (success && data) {
        setConfig(data);
      } else {
        setError(message || '加载网关配置失败');
      }
    } catch (e) {
      setError(e.message || '加载网关配置失败');
    } finally {
      setLoading(false);
    }
  }, []);

  const loadDeploymentStatuses = useCallback(async () => {
    try {
      const res = await API.get('/api/fallback/deployments/runtime-status');
      const { success, data } = res.data || {};
      if (success && data) {
        setDeploymentStatuses(data);
      }
    } catch (e) {
      // 静默失败，状态数据可选
    }
  }, []);

  useEffect(() => {
    loadConfig().then(() => {
      loadDeploymentStatuses();
    });
  }, [loadConfig, loadDeploymentStatuses]);

  const toggleVirtualModel = (vmKey) => {
    setExpandedVirtualModels((prev) => ({
      ...prev,
      [vmKey]: !prev[vmKey],
    }));
  };

  const toggleDeployment = (depKey) => {
    setExpandedDeployments((prev) => ({
      ...prev,
      [depKey]: !prev[depKey],
    }));
  };

  if (loading) {
    return (
      <div className='fallback-config-panel fallback-config-loading'>
        <Loader active inline='centered' />
      </div>
    );
  }

  if (error) {
    return (
      <div className='fallback-config-panel'>
        <Message negative>
          <Message.Header>加载失败</Message.Header>
          <p>{error}</p>
          <Button onClick={loadConfig} basic>
            重试
          </Button>
        </Message>
      </div>
    );
  }

  if (!config) {
    return (
      <div className='fallback-config-panel'>
        <Message warning>未加载到网关配置</Message>
        <Button onClick={loadConfig} loading={loading}>
          <Icon name='refresh' />
          重新加载
        </Button>
      </div>
    );
  }

  return (
    <div className='fallback-config-panel'>
      <div className='fallback-config-toolbar'>
        <div>
          <Header as='h3' className='fallback-config-title'>
            虚拟模型
          </Header>
          <div className='fallback-config-count'>
            {vmArray.length} 个虚拟模型，{deploymentArray.length} 个真实模型
          </div>
        </div>
        <div className='fallback-config-actions'>
          <Button icon labelPosition='left' onClick={loadConfig} loading={loading}>
            <Icon name='refresh' />
            刷新
          </Button>
        </div>
      </div>

      <Divider />

      <div className='fallback-virtual-list'>
        {vmArray.map((vm) => {
          const vmKey = vm.name;
          const vmExpanded = !!expandedVirtualModels[vmKey];
          const modelCount = (vm.fallback_order || []).length;
          const routingMode = vm.routing_mode || ROUTING_MODE_WEIGHTED;
          const isSequentialMode = routingMode === ROUTING_MODE_SEQUENTIAL;
          const isFixedMode = routingMode === ROUTING_MODE_FIXED;
          const routingMeta = ROUTING_MODE_META[routingMode] || ROUTING_MODE_META[ROUTING_MODE_WEIGHTED];

          return (
            <section className='fallback-virtual-panel' key={vmKey}>
              <div className='fallback-virtual-summary'>
                <Button
                  type='button'
                  basic
                  circular
                  className='fallback-collapse-button'
                  icon={vmExpanded ? 'angle down' : 'angle right'}
                  onClick={() => toggleVirtualModel(vmKey)}
                />
                <div className='fallback-virtual-summary-main'>
                  <div className='fallback-virtual-name'>
                    {vm.name || '未命名虚拟模型'}
                    {vm.name === 'cct/high' && (
                      <Label basic size='mini' color='blue'>高质量模型</Label>
                    )}
                    {vm.name === 'cct/low' && (
                      <Label basic size='mini' color='teal'>低成本模型</Label>
                    )}
                  </div>
                  <div className='fallback-virtual-meta'>
                    {modelCount} 个真实模型
                    {vm.description ? ` - ${vm.description}` : ''}
                    <Label basic size='mini' color={routingMeta.color}>
                      <Icon name={routingMeta.icon} /> {routingMeta.title}
                    </Label>
                  </div>
                </div>
                <div className='fallback-virtual-summary-actions'>
                  <Label basic color={vm.enabled ? 'green' : 'grey'}>
                    {vm.enabled ? '启用' : '停用'}
                  </Label>
                </div>
              </div>

              {vmExpanded && (
                <div className='fallback-virtual-body'>
                  <div className='fallback-virtual-header'>
                    <div style={{ fontSize: 13, color: '#667085', marginBottom: 8 }}>
                      {routingMeta.detail}
                      {vm.allow_degrade_to_low && ' · 可降级到低成本模型'}
                      {vm.allow_degrade_to_free && ' · 可降级到免费模型'}
                    </div>
                  </div>

                  <div className='fallback-deployment-list'>
                    {(vm.fallback_order || []).map((deploymentId, orderIndex) => {
                      const dep = deploymentsById[deploymentId];
                      if (!dep) return null;

                      const deploymentKey = `${vmKey}-${deploymentId}`;
                      const depExpanded = !!expandedDeployments[deploymentKey];
                      const deploymentStatus = deploymentStatuses[deploymentId];
                      const statusMeta = getDeploymentStatusMeta(deploymentStatus);
                      const isFixedDeployment = isFixedMode && vm.fixed_deployment === deploymentId;
                      const ownerNames = getDeploymentOwnerNames(
                        vmArray,
                        deploymentId
                      );
                      const ownerText = ownerNames.join(' / ');

                      return (
                        <div
                          className={`fallback-deployment-panel ${isFixedDeployment ? 'fixed-active' : ''} ${highlightDeployment === deploymentId ? 'fallback-highlight' : ''}`}
                          key={deploymentKey}
                        >
                          <div className='fallback-deployment-heading'>
                            <Button
                              type='button'
                              basic
                              circular
                              className='fallback-collapse-button'
                              icon={depExpanded ? 'angle down' : 'angle right'}
                              onClick={() => toggleDeployment(deploymentKey)}
                            />
                            <div className='fallback-deployment-name'>
                              {dep.real_model || '未命名真实模型'}
                              <Label
                                basic
                                size='mini'
                                color={
                                  isFixedMode
                                    ? 'purple'
                                    : isSequentialMode
                                      ? 'teal'
                                      : 'blue'
                                }
                              >
                                {isFixedMode
                                  ? isFixedDeployment
                                    ? '固定目标'
                                    : '候选'
                                  : isSequentialMode
                                    ? `顺序 #${orderIndex + 1}`
                                    : `权重 ${dep.weight || 100}`}
                              </Label>
                              <Label basic size='mini' color={statusMeta.color}>
                                {statusMeta.label}
                              </Label>
                              <Label basic size='mini' color={dep.quota_mode === 'free' ? 'blue' : 'teal'}>
                                {dep.quota_mode === 'free' ? '用完即换' : '限额 ' + (dep.daily_limit_tokens || 0).toLocaleString()}
                              </Label>
                              {ownerNames.length > 1 && (
                                <Label basic size='mini' color='orange' title={`共享部署：${ownerText}`}>
                                  共享 {ownerNames.length}
                                </Label>
                              )}
                            </div>
                          </div>

                          <div className='fallback-state-note'>
                            {statusMeta.detail}
                          </div>

                          {depExpanded && (
                            <div className='fallback-deployment-details'>
                              <Table compact celled size='small'>
                                <Table.Body>
                                  <Table.Row>
                                    <Table.Cell width={3}>部署 ID</Table.Cell>
                                    <Table.Cell><code>{dep.id}</code></Table.Cell>
                                  </Table.Row>
                                  <Table.Row>
                                    <Table.Cell>渠道 ID</Table.Cell>
                                    <Table.Cell>{dep.channel_id || '-'}</Table.Cell>
                                  </Table.Row>
                                  <Table.Row>
                                    <Table.Cell>池</Table.Cell>
                                    <Table.Cell>{dep.pool || '-'}</Table.Cell>
                                  </Table.Row>
                                  <Table.Row>
                                    <Table.Cell>质量层级</Table.Cell>
                                    <Table.Cell>{dep.quality_tier || '-'}</Table.Cell>
                                  </Table.Row>
                                  <Table.Row>
                                    <Table.Cell>成本层级</Table.Cell>
                                    <Table.Cell>{dep.cost_tier || '-'}</Table.Cell>
                                  </Table.Row>
                                  <Table.Row>
                                    <Table.Cell>优先级</Table.Cell>
                                    <Table.Cell>{dep.priority || '-'}</Table.Cell>
                                  </Table.Row>
                                  <Table.Row>
                                    <Table.Cell>权重</Table.Cell>
                                    <Table.Cell>{dep.weight || '-'}</Table.Cell>
                                  </Table.Row>
                                  <Table.Row>
                                    <Table.Cell>上下文长度</Table.Cell>
                                    <Table.Cell>{dep.context_length || '-'}</Table.Cell>
                                  </Table.Row>
                                  <Table.Row>
                                    <Table.Cell>RPM 限额</Table.Cell>
                                    <Table.Cell>{dep.rpm_limit || '-'}</Table.Cell>
                                  </Table.Row>
                                  <Table.Row>
                                    <Table.Cell>RPD 限额</Table.Cell>
                                    <Table.Cell>{dep.rpd_limit || '-'}</Table.Cell>
                                  </Table.Row>
                                  <Table.Row>
                                    <Table.Cell>TPM 限额</Table.Cell>
                                    <Table.Cell>{dep.tpm_limit || '-'}</Table.Cell>
                                  </Table.Row>
                                  <Table.Row>
                                    <Table.Cell>TPD 限额</Table.Cell>
                                    <Table.Cell>{dep.tpd_limit || '-'}</Table.Cell>
                                  </Table.Row>
                                  <Table.Row>
                                    <Table.Cell>能力</Table.Cell>
                                    <Table.Cell>
                                      {dep.supports_vision && <Label size='tiny' color='blue'>Vision</Label>}
                                      {dep.supports_stream && <Label size='tiny' color='teal'>Stream</Label>}
                                      {dep.supports_tools && <Label size='tiny' color='purple'>Tools</Label>}
                                      {dep.supports_json && <Label size='tiny' color='orange'>JSON</Label>}
                                      {!dep.supports_vision && !dep.supports_stream && !dep.supports_tools && !dep.supports_json && '-'}
                                    </Table.Cell>
                                  </Table.Row>
                                </Table.Body>
                              </Table>
                            </div>
                          )}
                        </div>
                      );
                    })}
                  </div>
                </div>
              )}
            </section>
          );
        })}
      </div>
    </div>
  );
};

export default ModelEditor;
