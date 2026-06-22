import React, { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Button,
  Checkbox,
  Divider,
  Dropdown,
  Header,
  Icon,
  Input,
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

const slugModelName = (name) =>
  String(name || '').toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/-+/g, '-').replace(/^-|-$/g, '');

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
  const [saving, setSaving] = useState(false);
  const [draftDeployments, setDraftDeployments] = useState({});
  const [saveMessage, setSaveMessage] = useState(null);
  const [channels, setChannels] = useState([]);
  const [selectorState, setSelectorState] = useState({});
  const [healthTesting, setHealthTesting] = useState({});
  const [healthResults, setHealthResults] = useState({});
  const [showAddVM, setShowAddVM] = useState(false);
  const [newVMName, setNewVMName] = useState('');
  const [newVMStrategy, setNewVMStrategy] = useState('weighted');
  const [newVMPool, setNewVMPool] = useState('');

  const HIDDEN_VMS = ['cct/free'];
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
    return Object.keys(config.virtual_models)
      .filter((name) => !HIDDEN_VMS.includes(name))
      .map((name) => {
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

  const handleHealthCheck = useCallback(async (deploymentId) => {
    setHealthTesting((prev) => ({ ...prev, [deploymentId]: true }));
    setHealthResults((prev) => ({ ...prev, [deploymentId]: null }));
    try {
      const res = await API.post(`/api/fallback/deployments/${deploymentId}/health-check`);
      const { success, data, message } = res.data || {};
      const healthStatus = data?.health || (success ? 'healthy' : 'error');
      setHealthResults((prev) => ({
        ...prev,
        [deploymentId]: { ok: success !== false, text: message || healthStatus },
      }));
      await loadDeploymentStatuses();
    } catch (e) {
      setHealthResults((prev) => ({
        ...prev,
        [deploymentId]: { ok: false, text: e.message || '请求失败' },
      }));
    } finally {
      setHealthTesting((prev) => ({ ...prev, [deploymentId]: false }));
    }
  }, [loadDeploymentStatuses]);

  const loadChannels = useCallback(async () => {
    try {
      const allChannels = [];
      let page = 0;
      let hasMore = true;
      while (hasMore) {
        const res = await API.get(`/api/channel/?p=${page}`);
        const { success, data } = res.data || {};
        if (!success || !Array.isArray(data) || data.length === 0) {
          hasMore = false;
          break;
        }
        allChannels.push(...data);
        if (data.length < 10) {
          hasMore = false;
        } else {
          page++;
        }
      }
      // Parse models from comma-separated string to array
      const parsed = allChannels.map((ch) => ({
        id: ch.id,
        name: ch.name || `渠道 ${ch.id}`,
        models: (ch.models || '').split(',').map((m) => m.trim()).filter(Boolean),
      }));
      setChannels(parsed);
    } catch (e) {
      // 静默失败，渠道数据可选
    }
  }, []);

  // Existing (channel_id, real_model) pairs in non-free deployments
  const existingPairs = useMemo(() => {
    const pairs = new Set();
    if (!config?.deployments) return pairs;
    Object.entries(config.deployments).forEach(([id, dep]) => {
      if (isSeparatorKey(id)) return;
      if (dep?.pool === 'free' || isFreeDeployment(id)) return;
      if (dep?.channel_id && dep?.real_model) {
        pairs.add(`${dep.channel_id}:${dep.real_model}`);
      }
    });
    return pairs;
  }, [config]);

  const setDraftField = (depId, field, value) => {
    setDraftDeployments((prev) => ({
      ...prev,
      [depId]: {
        ...prev[depId],
        [field]: value,
      },
    }));
  };

  const handleSave = useCallback(async () => {
    setSaving(true);
    setSaveMessage(null);
    try {
      // Step 1: re-fetch fresh config (never use stale snapshot)
      const freshRes = await API.get('/api/fallback/gateway/config');
      const fresh = freshRes.data?.data;
      if (!fresh) {
        setSaveMessage({ type: 'error', text: '无法获取最新配置，保存中止' });
        return;
      }

      // Step 2: deep-clone fresh as payload
      const payload = JSON.parse(JSON.stringify(fresh));

      // Step 3: overlay draft edits onto non-free, non-separator deployments
      // Only edit enabled/priority/weight — never touch strategy/pools or
      // carry-over fields (daily_limit_tokens, soft/hard_limit_ratio, etc.)
      if (payload.deployments) {
        Object.keys(payload.deployments).forEach((id) => {
          // skip separator keys
          if (isSeparatorKey(id)) return;
          // skip free deployments — never touch them
          const dep = payload.deployments[id];
          if (dep?.pool === 'free' || isFreeDeployment(id)) return;
          // apply draft edits if any exist for this deployment
          const draft = draftDeployments[id];
          if (draft) {
            if (draft.enabled !== undefined) {
              dep.enabled = draft.enabled;
            }
            if (draft.priority !== undefined) {
              dep.priority = Number(draft.priority);
            }
            if (draft.weight !== undefined) {
              dep.weight = Number(draft.weight);
            }
            payload.deployments[id] = dep;
          }
        });
      }

      // Step 4: PUT the merged payload
      // virtual_models, free_providers, free deployments all remain as-is from fresh
      const putRes = await API.put('/api/fallback/gateway/config', payload);
      const { success, message } = putRes.data || {};
      if (success) {
        setSaveMessage({ type: 'success', text: '保存成功' });
        setDraftDeployments({});
        // reload to reflect the saved state
        await loadConfig();
        await loadDeploymentStatuses();
      } else {
        setSaveMessage({ type: 'error', text: message || '保存失败' });
      }
    } catch (e) {
      setSaveMessage({ type: 'error', text: e.message || '保存异常' });
    } finally {
      setSaving(false);
    }
  }, [draftDeployments, loadConfig, loadDeploymentStatuses]);

  const handleAddDeployment = useCallback(async (channelId, model, pool, vmKey) => {
    setSaving(true);
    setSaveMessage(null);
    try {
      const freshRes = await API.get('/api/fallback/gateway/config');
      const fresh = freshRes.data?.data;
      if (!fresh) {
        setSaveMessage({ type: 'error', text: '无法获取最新配置，添加中止' });
        return;
      }
      const payload = JSON.parse(JSON.stringify(fresh));
      if (!payload.deployments) payload.deployments = {};
      // Duplicate check: (channel_id, real_model) pair
      for (const [, dep] of Object.entries(payload.deployments)) {
        if (dep?.channel_id === channelId && dep?.real_model === model) {
          setSaveMessage({ type: 'error', text: `该渠道已有模型 ${model}，不可重复添加` });
          return;
        }
      }
      // Generate unique ID
      let baseId = `manual-${channelId}-${slugModelName(model)}`;
      if (baseId.startsWith('free:') || baseId.startsWith('---')) {
        baseId = `m-${channelId}-${slugModelName(model)}`;
      }
      let newId = baseId;
      let suffix = 1;
      while (payload.deployments[newId]) {
        newId = `${baseId}-${suffix}`;
        suffix++;
      }
      payload.deployments[newId] = {
        enabled: true,
        channel_id: channelId,
        real_model: model,
        pool: pool,
        priority: 0,
        weight: 100,
      };
      const putRes = await API.put('/api/fallback/gateway/config', payload);
      const { success, message } = putRes.data || {};
      if (success) {
        setSaveMessage({ type: 'success', text: `已添加部署 ${newId}` });
        setSelectorState((prev) => ({ ...prev, [vmKey]: null }));
        await loadConfig();
        await loadDeploymentStatuses();
      } else {
        setSaveMessage({ type: 'error', text: message || '添加失败' });
      }
    } catch (e) {
      setSaveMessage({ type: 'error', text: e.message || '添加异常' });
    } finally {
      setSaving(false);
    }
  }, [loadConfig, loadDeploymentStatuses]);

  const handleDeleteDeployment = useCallback(async (deploymentId) => {
    if (!deploymentId) return;
    if (isFreeDeployment(deploymentId)) {
      setSaveMessage({ type: 'error', text: '免费部署不可在模型编辑器中删除' });
      return;
    }
    // Pool safety check
    const currentDep = config?.deployments?.[deploymentId];
    if (currentDep?.pool) {
      const pool = currentDep.pool;
      const enabledInPool = Object.entries(config.deployments).filter(
        ([id, d]) => !isSeparatorKey(id) && !isFreeDeployment(id) && d?.pool === pool && d?.enabled !== false && id !== deploymentId
      );
      if (enabledInPool.length === 0) {
        const warnMsg = `删除后池 "${pool}" 将没有可用部署，相关虚拟模型可能无法路由。\n确定要继续吗？`;
        if (!window.confirm(warnMsg)) return;
      }
    }
    setSaving(true);
    setSaveMessage(null);
    try {
      const freshRes = await API.get('/api/fallback/gateway/config');
      const fresh = freshRes.data?.data;
      if (!fresh) {
        setSaveMessage({ type: 'error', text: '无法获取最新配置，删除中止' });
        return;
      }
      const payload = JSON.parse(JSON.stringify(fresh));
      if (!payload.deployments?.[deploymentId]) {
        setSaveMessage({ type: 'error', text: `部署 ${deploymentId} 不存在于最新配置中` });
        return;
      }
      delete payload.deployments[deploymentId];
      const putRes = await API.put('/api/fallback/gateway/config', payload);
      const { success, message } = putRes.data || {};
      if (success) {
        setSaveMessage({ type: 'success', text: `已删除部署 ${deploymentId}` });
        setDraftDeployments((prev) => {
          const next = { ...prev };
          delete next[deploymentId];
          return next;
        });
        await loadConfig();
        await loadDeploymentStatuses();
      } else {
        setSaveMessage({ type: 'error', text: message || '删除失败' });
      }
    } catch (e) {
      setSaveMessage({ type: 'error', text: e.message || '删除异常' });
    } finally {
      setSaving(false);
    }
  }, [config, loadConfig, loadDeploymentStatuses]);

  const handleAddVirtualModel = useCallback(async () => {
    const name = newVMName.trim();
    if (!name) {
      setSaveMessage({ type: 'error', text: '虚拟模型名称不能为空' });
      return;
    }
    if (name.startsWith('cct/')) {
      // cct/ prefix is reserved for system VMs
      setSaveMessage({ type: 'error', text: 'cct/ 前缀保留给系统虚拟模型，请用其他名称' });
      return;
    }
    const pool = newVMPool.trim() || 'default';
    setSaving(true);
    setSaveMessage(null);
    try {
      const freshRes = await API.get('/api/fallback/gateway/config');
      const fresh = freshRes.data?.data;
      if (!fresh) {
        setSaveMessage({ type: 'error', text: '无法获取最新配置，添加中止' });
        return;
      }
      if (fresh.virtual_models?.[name]) {
        setSaveMessage({ type: 'error', text: `虚拟模型 ${name} 已存在` });
        return;
      }
      const payload = JSON.parse(JSON.stringify(fresh));
      if (!payload.virtual_models) payload.virtual_models = {};
      payload.virtual_models[name] = {
        enabled: true,
        strategy: newVMStrategy,
        pools: [pool],
        allow_degrade_to_low: false,
        allow_degrade_to_free: false,
      };
      const putRes = await API.put('/api/fallback/gateway/config', payload);
      const { success, message } = putRes.data || {};
      if (success) {
        setSaveMessage({ type: 'success', text: `已添加虚拟模型 ${name}` });
        setNewVMName('');
        setNewVMStrategy('weighted');
        setNewVMPool('');
        setShowAddVM(false);
        await loadConfig();
        await loadDeploymentStatuses();
      } else {
        setSaveMessage({ type: 'error', text: message || '添加失败' });
      }
    } catch (e) {
      setSaveMessage({ type: 'error', text: e.message || '添加异常' });
    } finally {
      setSaving(false);
    }
  }, [newVMName, newVMStrategy, newVMPool, loadConfig, loadDeploymentStatuses]);

  const handleDeleteVirtualModel = useCallback(async (vmName) => {
    if (!vmName || HIDDEN_VMS.includes(vmName)) return;
    if (vmName.startsWith('cct/')) {
      setSaveMessage({ type: 'error', text: '系统虚拟模型不可删除' });
      return;
    }
    if (!window.confirm(`确定要删除虚拟模型 ${vmName} 吗？`)) return;
    setSaving(true);
    setSaveMessage(null);
    try {
      const freshRes = await API.get('/api/fallback/gateway/config');
      const fresh = freshRes.data?.data;
      if (!fresh) {
        setSaveMessage({ type: 'error', text: '无法获取最新配置，删除中止' });
        return;
      }
      if (!fresh.virtual_models?.[vmName]) {
        setSaveMessage({ type: 'error', text: `虚拟模型 ${vmName} 不存在于最新配置中` });
        return;
      }
      const payload = JSON.parse(JSON.stringify(fresh));
      delete payload.virtual_models[vmName];
      const putRes = await API.put('/api/fallback/gateway/config', payload);
      const { success, message } = putRes.data || {};
      if (success) {
        setSaveMessage({ type: 'success', text: `已删除虚拟模型 ${vmName}` });
        await loadConfig();
        await loadDeploymentStatuses();
      } else {
        setSaveMessage({ type: 'error', text: message || '删除失败' });
      }
    } catch (e) {
      setSaveMessage({ type: 'error', text: e.message || '删除异常' });
    } finally {
      setSaving(false);
    }
  }, [loadConfig, loadDeploymentStatuses]);

  useEffect(() => {
    loadConfig().then(() => {
      loadDeploymentStatuses();
    });
    loadChannels();
  }, [loadConfig, loadDeploymentStatuses, loadChannels]);

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
          <Button icon labelPosition='left' onClick={handleSave} loading={saving} disabled={!config}>
            <Icon name='save' />
            保存
          </Button>
          <Button icon labelPosition='left' onClick={loadConfig} loading={loading}>
            <Icon name='refresh' />
            刷新
          </Button>
        </div>
      </div>

      {saveMessage && (
        <Message
          positive={saveMessage.type === 'success'}
          negative={saveMessage.type === 'error'}
          onDismiss={() => setSaveMessage(null)}
          style={{ marginTop: 12 }}
        >
          <p>{saveMessage.text}</p>
        </Message>
      )}

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
                    {' · '}
                    <Label basic size='mini' color={routingMeta.color}>
                      <Icon name={routingMeta.icon} /> {routingMeta.title}
                    </Label>
                  </div>
                </div>
                <div className='fallback-virtual-summary-actions'>
                  <Label basic color={vm.enabled ? 'green' : 'grey'}>
                    {vm.enabled ? '启用' : '停用'}
                  </Label>
                  {!vm.name.startsWith('cct/') && (
                    <Button
                      size='mini'
                      negative
                      icon
                      labelPosition='left'
                      loading={saving}
                      onClick={() => handleDeleteVirtualModel(vm.name)}
                    >
                      <Icon name='trash' />
                      删除虚拟模型
                    </Button>
                  )}
                </div>
              </div>

              {vmExpanded && (
                <div className='fallback-virtual-body'>
                  {/* Model Selector */}
                  <div style={{ marginBottom: 16, padding: '12px', background: '#f8fafc', borderRadius: 8, border: '1px dashed #d9e0ea' }}>
                    <div style={{ fontSize: 13, fontWeight: 600, color: '#475569', marginBottom: 8 }}>
                      <Icon name='plus circle' /> 添加真实模型
                    </div>
                    <Dropdown
                      placeholder='选择渠道和模型...'
                      fluid
                      search
                      selection
                      value={selectorState[vmKey]?.value || ''}
                      options={(() => {
                        const opts = [];
                        let lastChannelId = null;
                        channels.forEach((ch) => {
                          // Channel group header
                          opts.push({
                            key: `header-${ch.id}`,
                            text: `${ch.name} (${ch.id})`,
                            value: `__header_${ch.id}__`,
                            disabled: true,
                            className: 'fallback-channel-header',
                            content: (
                              <div style={{ fontWeight: 700, color: '#172033', padding: '4px 0', fontSize: 13 }}>
                                {ch.name} <span style={{ color: '#98a2b3', fontWeight: 400 }}>#{ch.id}</span>
                              </div>
                            ),
                          });
                          ch.models.forEach((model) => {
                            const pairKey = `${ch.id}:${model}`;
                            const exists = existingPairs.has(pairKey);
                            opts.push({
                              key: pairKey,
                              text: `#${ch.id} ${model}${exists ? ' (已添加)' : ''}`,
                              value: pairKey,
                              disabled: exists,
                              description: exists ? '已存在' : '',
                            });
                          });
                        });
                        return opts;
                      })()}
                      onChange={(_, { value }) => {
                        if (!value || String(value).startsWith('__header_')) return;
                        const [channelIdStr, ...modelParts] = String(value).split(':');
                        const channelId = Number(channelIdStr);
                        const model = modelParts.join(':');
                        setSelectorState((prev) => ({
                          ...prev,
                          [vmKey]: { value, channelId, model, pool: vm.pools?.[0] || 'default' },
                        }));
                      }}
                    />

                    {/* Preview card */}
                    {selectorState[vmKey] && (() => {
                      const sel = selectorState[vmKey];
                      return (
                        <div style={{ marginTop: 10, padding: '10px 12px', background: '#fff', border: '1px solid #d9e0ea', borderRadius: 6, fontSize: 13 }}>
                          <div style={{ display: 'flex', gap: 16, flexWrap: 'wrap', alignItems: 'center' }}>
                            <span><strong>渠道:</strong> <span style={{ color: '#98a2b3' }}>#{sel.channelId}</span></span>
                            <span><strong>模型:</strong> {sel.model}</span>
                            <Button
                              size='mini'
                              color='blue'
                              icon
                              labelPosition='left'
                              loading={saving}
                              disabled={!selectorState[vmKey]}
                              onClick={() => {
                                const s = selectorState[vmKey];
                                if (!s) return;
                                handleAddDeployment(s.channelId, s.model, s.pool, vmKey);
                              }}
                              style={{ marginLeft: 'auto' }}
                            >
                              <Icon name='plus' />
                              添加
                            </Button>
                          </div>
                        </div>
                      );
                    })()}
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
                                    : `权重 ${draftDeployments[dep.id]?.weight ?? dep.weight ?? 100}`}
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
                                    <Table.Cell>渠道 ID</Table.Cell>
                                    <Table.Cell>{dep.channel_id || '-'}</Table.Cell>
                                  </Table.Row>
                                  <Table.Row>
                                    <Table.Cell>启用</Table.Cell>
                                    <Table.Cell>
                                      <Checkbox
                                        toggle
                                        checked={
                                          draftDeployments[dep.id]?.enabled !== undefined
                                            ? draftDeployments[dep.id].enabled
                                            : dep.enabled !== false
                                        }
                                        onChange={(_, { checked }) =>
                                          setDraftField(dep.id, 'enabled', checked)
                                        }
                                      />
                                    </Table.Cell>
                                  </Table.Row>
                                  <Table.Row>
                                    <Table.Cell>优先级</Table.Cell>
                                    <Table.Cell>
                                      <Input
                                        type='number'
                                        size='mini'
                                        style={{ maxWidth: 100 }}
                                        value={
                                          draftDeployments[dep.id]?.priority !== undefined
                                            ? draftDeployments[dep.id].priority
                                            : dep.priority ?? 0
                                        }
                                        onChange={(_, { value }) =>
                                          setDraftField(dep.id, 'priority', value)
                                        }
                                      />
                                    </Table.Cell>
                                  </Table.Row>
                                  <Table.Row>
                                    <Table.Cell>权重</Table.Cell>
                                    <Table.Cell>
                                      <Input
                                        type='number'
                                        size='mini'
                                        style={{ maxWidth: 100 }}
                                        value={
                                          draftDeployments[dep.id]?.weight !== undefined
                                            ? draftDeployments[dep.id].weight
                                            : dep.weight ?? 100
                                        }
                                        onChange={(_, { value }) =>
                                          setDraftField(dep.id, 'weight', value)
                                        }
                                      />
                                    </Table.Cell>
                                  </Table.Row>
                                  <Table.Row>
                                    <Table.Cell>操作</Table.Cell>
                                    <Table.Cell>
                                      <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
                                        <Button
                                          size='mini'
                                          icon
                                          labelPosition='left'
                                          loading={healthTesting[dep.id]}
                                          disabled={healthTesting[dep.id] || saving}
                                          onClick={() => handleHealthCheck(dep.id)}
                                        >
                                          <Icon name='heartbeat' />
                                          连通性测试
                                        </Button>
                                        {healthResults[dep.id] && (
                                          <Label basic size='mini' color={healthResults[dep.id].ok ? 'green' : 'red'}>
                                            <Icon name={healthResults[dep.id].ok ? 'check' : 'times'} />
                                            {healthResults[dep.id].text}
                                          </Label>
                                        )}
                                        <Button
                                          size='mini'
                                          negative
                                          icon
                                          labelPosition='left'
                                          loading={saving}
                                          onClick={() => handleDeleteDeployment(dep.id)}
                                        >
                                          <Icon name='trash' />
                                          删除此部署
                                        </Button>
                                      </div>
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

      {/* Add Virtual Model */}
      <div style={{ marginTop: 16 }}>
        {!showAddVM ? (
          <Button
            icon
            labelPosition='left'
            onClick={() => setShowAddVM(true)}
          >
            <Icon name='plus' />
            添加虚拟模型
          </Button>
        ) : (
          <div style={{ padding: 16, background: '#f8fafc', borderRadius: 8, border: '1px dashed #d9e0ea' }}>
            <div style={{ fontSize: 13, fontWeight: 600, color: '#475569', marginBottom: 12 }}>
              <Icon name='plus circle' /> 新建虚拟模型
            </div>
            <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap', alignItems: 'flex-end' }}>
              <div>
                <label style={{ display: 'block', fontSize: 12, color: '#64748b', marginBottom: 4 }}>名称</label>
                <Input
                  size='small'
                  placeholder='例: my-model'
                  value={newVMName}
                  onChange={(_, { value }) => setNewVMName(value)}
                  style={{ width: 200 }}
                />
              </div>
              <div>
                <label style={{ display: 'block', fontSize: 12, color: '#64748b', marginBottom: 4 }}>路由策略</label>
                <Dropdown
                  size='small'
                  selection
                  value={newVMStrategy}
                  options={[
                    { key: 'weighted', text: '按权重', value: 'weighted' },
                    { key: 'quality_first', text: '质量优先（顺序）', value: 'quality_first' },
                    { key: 'cost_first', text: '成本优先（顺序）', value: 'cost_first' },
                    { key: 'fixed', text: '固定模型', value: 'fixed' },
                  ]}
                  onChange={(_, { value }) => setNewVMStrategy(value)}
                />
              </div>
              <div>
                <label style={{ display: 'block', fontSize: 12, color: '#64748b', marginBottom: 4 }}>池名称</label>
                <Input
                  size='small'
                  placeholder='默认: default'
                  value={newVMPool}
                  onChange={(_, { value }) => setNewVMPool(value)}
                  style={{ width: 160 }}
                />
              </div>
              <div style={{ display: 'flex', gap: 8 }}>
                <Button
                  color='blue'
                  size='small'
                  icon
                  labelPosition='left'
                  loading={saving}
                  disabled={!newVMName.trim()}
                  onClick={handleAddVirtualModel}
                >
                  <Icon name='check' />
                  确认添加
                </Button>
                <Button
                  size='small'
                  onClick={() => { setShowAddVM(false); setNewVMName(''); setNewVMPool(''); }}
                >
                  取消
                </Button>
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  );
};

export default ModelEditor;
