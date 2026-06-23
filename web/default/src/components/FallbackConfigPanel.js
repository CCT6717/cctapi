import React, { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Button,
  Divider,
  Dropdown,
  Header,
  Icon,
  Label,
  Loader,
  Message,
} from 'semantic-ui-react';
import { API } from '../helpers';
import { buildSavePayload } from './utils/savePipeline';
import BaseUrlModal from './modals/BaseUrlModal';
import KeyModal from './modals/KeyModal';
import AddVirtualModelPanel from './modals/AddVirtualModelPanel';
import DeploymentRow from './deployments/DeploymentRow';
import { useGatewayConfig } from './hooks/useGatewayConfig';
import { useDeploymentStatuses } from './hooks/useDeploymentStatuses';
import { useChannels } from './hooks/useChannels';
import {
  isSeparatorKey,
  isFreeDeployment,
  slugModelName,
  computeInitialMode,
  getDeploymentStatusMeta,
  getDeploymentOwnerNames,
} from './utils/deploymentMeta';
import './FallbackConfigPanel.css';

const ModelEditor = ({ highlightDeployment }) => {
  // Gateway config + deployment modes (modes initialised from config)
  const { config, loading, error, loadConfig, deploymentMode, setDeploymentMode } = useGatewayConfig();
  // Runtime statuses (optional, silent fail)
  const { deploymentStatuses, loadDeploymentStatuses } = useDeploymentStatuses();
  // Channels for the model selector (optional, silent fail)
  const { channels, loadChannels } = useChannels();

  const [expandedVirtualModels, setExpandedVirtualModels] = useState({});
  const [expandedDeployments, setExpandedDeployments] = useState({});
  const [saving, setSaving] = useState(false);
  const [draftDeployments, setDraftDeployments] = useState({});
  const [draftRoutingVm, setDraftRoutingVm] = useState({}); // { [vmKey]: strategy }
  const [saveMessage, setSaveMessage] = useState(null);
  const [selectorState, setSelectorState] = useState({});
  const [healthTesting, setHealthTesting] = useState({});
  const [healthResults, setHealthResults] = useState({});
  const [showAddVM, setShowAddVM] = useState(false);
  const [newVMName, setNewVMName] = useState('');
  const [newVMStrategy, setNewVMStrategy] = useState('quality_first');
  const [newVMPool, setNewVMPool] = useState('');
  const [baseUrlModal, setBaseUrlModal] = useState(null); // { channelId, baseUrl, saving, error }
  const [keyModal, setKeyModal] = useState(null); // { channelId, newKey, showPlain, saving, error }

  const HIDDEN_VMS = ['cct/free'];

  const visibleDeploymentIds = useMemo(() => {
    if (!config?.deployments) return [];
    return Object.keys(config.deployments).filter(
      (id) => {
        if (isSeparatorKey(id)) return false;
        const dep = config.deployments[id];
        if (isFreeDeployment(id, dep)) return false;
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
      return {
        name,
        ...vm,
        fallback_order: fallbackOrder,
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

  // Existing (channel_id, real_model) pairs in non-free deployments
  const existingPairs = useMemo(() => {
    const pairs = new Set();
    if (!config?.deployments) return pairs;
    Object.entries(config.deployments).forEach(([id, dep]) => {
      if (isSeparatorKey(id)) return;
      if (isFreeDeployment(id, dep)) return;
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

  // { depId: vmKey } — which VM "owns" each deployment
  const deploymentOwnerVm = useMemo(() => {
    const map = {};
    if (config?.virtual_models) {
      Object.entries(config.virtual_models).forEach(([vmKey, vm]) => {
        (vm.pools || []).forEach((pool) => {
          if (config.deployments) {
            Object.entries(config.deployments).forEach(([depId, dep]) => {
              if (dep.pool === pool) map[depId] = vmKey;
            });
          }
        });
      });
    }
    return map;
  }, [config]);

  const handleRoutingModeChange = useCallback((vmKey, strategy) => {
    setDraftRoutingVm((prev) => ({ ...prev, [vmKey]: strategy }));
  }, []);

  const handleModeChange = useCallback((depId, mode, vmKey) => {
    setDeploymentMode((prev) => {
      const next = { ...prev, [depId]: mode };
      if (mode === 'fixed') {
        // un-fix other deployments in the same VM (only one fixed per VM)
        Object.keys(next).forEach((id) => {
          if (id !== depId && next[id] === 'fixed' && deploymentOwnerVm[id] === vmKey) {
            next[id] = 'error';
          }
        });
      }
      return next;
    });
    if (mode === 'quota') {
      setDraftDeployments((prev) => {
        const cur = prev[depId];
        if (!cur || cur.daily_limit_tokens === undefined || cur.daily_limit_tokens <= 0) {
          return { ...prev, [depId]: { ...cur, daily_limit_tokens: 100000 } };
        }
        return prev;
      });
    }
    if (mode === 'error') {
      setDraftDeployments((prev) => ({
        ...prev,
        [depId]: { ...(prev[depId] || {}), daily_limit_tokens: 0 },
      }));
    }
  }, [deploymentOwnerVm, setDeploymentMode, setDraftDeployments]);

  const handleSave = useCallback(async () => {
    setSaving(true);
    setSaveMessage(null);
    try {
      // Step 1: re-fetch fresh config
      const freshRes = await API.get('/api/fallback/gateway/config');
      const fresh = freshRes.data?.data;
      if (!fresh) {
        setSaveMessage({ type: 'error', text: '无法获取最新配置，保存中止' });
        return;
      }

      // Steps 2-3.5: build payload from fresh config + all draft state
      const payload = buildSavePayload(fresh, { draftDeployments, draftRoutingVm, deploymentMode, deploymentOwnerVm });

      // Step 4: PUT
      const putRes = await API.put('/api/fallback/gateway/config', payload);
      const { success, message } = putRes.data || {};
      if (success) {
        setSaveMessage({ type: 'success', text: '保存成功' });
        setDraftDeployments({});
        setDraftRoutingVm({});
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
  }, [draftDeployments, draftRoutingVm, deploymentMode, deploymentOwnerVm, loadConfig, loadDeploymentStatuses]);

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
    const currentDep = config?.deployments?.[deploymentId];
    if (isFreeDeployment(deploymentId, currentDep)) {
      setSaveMessage({ type: 'error', text: '免费部署不可在模型编辑器中删除' });
      return;
    }
    // Pool safety check
    if (currentDep?.pool) {
      const pool = currentDep.pool;
      const enabledInPool = Object.entries(config.deployments).filter(
        ([id, d]) => !isSeparatorKey(id) && !isFreeDeployment(id, d) && d?.pool === pool && d?.enabled !== false && id !== deploymentId
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
        setNewVMStrategy('quality_first');
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

  const openBaseUrlEditor = useCallback(async (channelId) => {
    try {
      const res = await API.get(`/api/channel/${channelId}`);
      const ch = res.data?.data;
      if (ch) {
        setBaseUrlModal({ channelId, baseUrl: ch.base_url || '', saving: false, error: '' });
      } else {
        setSaveMessage({ type: 'error', text: '获取渠道信息失败' });
      }
    } catch (e) {
      setSaveMessage({ type: 'error', text: e.message || '获取渠道信息异常' });
    }
  }, []);

  const saveBaseUrl = useCallback(async () => {
    if (!baseUrlModal?.channelId) return;
    setBaseUrlModal((prev) => ({ ...prev, saving: true, error: '' }));
    try {
      // strip trailing slash for consistency
      let baseUrl = (baseUrlModal.baseUrl || '').trim();
      if (baseUrl.endsWith('/')) baseUrl = baseUrl.slice(0, -1);
      const res = await API.put('/api/channel/', {
        id: baseUrlModal.channelId,
        base_url: baseUrl,
      });
      if (res.data?.success) {
        setSaveMessage({ type: 'success', text: `渠道 #${baseUrlModal.channelId} base_url 已更新` });
        setBaseUrlModal(null);
      } else {
        setBaseUrlModal((prev) => ({
          ...prev,
          saving: false,
          error: res.data?.message || '保存失败',
        }));
      }
    } catch (e) {
      setBaseUrlModal((prev) => ({
        ...prev,
        saving: false,
        error: e.message || '保存异常',
      }));
    }
  }, [baseUrlModal]);

  // 编辑 key: GET /api/channel/:id 不返回原 key,只能用新值覆盖。
  // 安全说明: 新 key 通过 HTTPS 以明文 PUT,会话已是 cookie-authed admin;
  // 空输入不触发 PUT,避免误清空; 绝不留 "显示原 key" 入口 (服务端无此数据)。
  const openKeyEditor = useCallback((channelId) => {
    setKeyModal({ channelId, newKey: '', showPlain: false, saving: false, error: '' });
  }, []);

  const saveKey = useCallback(async () => {
    if (!keyModal?.channelId) return;
    if (!keyModal.newKey) {
      // 空输入:不发送 PUT,避免把现有 key 清空
      setKeyModal(null);
      return;
    }
    setKeyModal((prev) => ({ ...prev, saving: true, error: '' }));
    try {
      const res = await API.put('/api/channel/', {
        id: keyModal.channelId,
        key: keyModal.newKey,
      });
      if (res.data?.success) {
        setSaveMessage({ type: 'success', text: `渠道 #${keyModal.channelId} key 已更新` });
        setKeyModal(null);
      } else {
        setKeyModal((prev) => ({
          ...prev,
          saving: false,
          error: res.data?.message || '保存失败',
        }));
      }
    } catch (e) {
      setKeyModal((prev) => ({
        ...prev,
        saving: false,
        error: e.message || '保存异常',
      }));
    }
  }, [keyModal]);

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
                    <Dropdown
                      inline
                      labeled
                      selection
                      size='mini'
                      style={{ minWidth: 140 }}
                      value={draftRoutingVm[vmKey] ?? vm.strategy ?? 'quality_first'}
                      options={[
                        { key: 'quality_first', text: '质量优先', value: 'quality_first' },
                        { key: 'cost_first', text: '成本优先', value: 'cost_first' },
                        { key: 'free_first', text: '免费优先', value: 'free_first' },
                      ]}
                      onChange={(_, { value }) => handleRoutingModeChange(vmKey, value)}
                    />
                    {vm.fallback_order?.some((depId) => (deploymentMode[depId] || computeInitialMode(config, depId)) === 'fixed') && (
                      <Label basic size='mini' color='blue' style={{ marginLeft: 6 }}>固定模式</Label>
                    )}
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
                      const ownerNames = getDeploymentOwnerNames(vmArray, deploymentId);
                      const ownerText = ownerNames.join(' / ');
                      const currentMode = deploymentMode[dep.id] || computeInitialMode(config, dep.id);

                      return (
                        <DeploymentRow
                          key={deploymentKey}
                          dep={dep}
                          orderIndex={orderIndex}
                          expanded={depExpanded}
                          highlighted={highlightDeployment === deploymentId}
                          statusMeta={statusMeta}
                          ownerNames={ownerNames}
                          ownerText={ownerText}
                          vmKey={vmKey}
                          draftDeployments={draftDeployments}
                          currentMode={currentMode}
                          healthTesting={!!healthTesting[dep.id]}
                          healthResult={healthResults[dep.id] || null}
                          saving={saving}
                          onToggle={() => toggleDeployment(deploymentKey)}
                          onDraftField={(field, value) => setDraftField(dep.id, field, value)}
                          onModeChange={(mode) => handleModeChange(dep.id, mode, vmKey)}
                          onHealthCheck={() => handleHealthCheck(dep.id)}
                          onEditBaseUrl={() => openBaseUrlEditor(dep.channel_id)}
                          onEditKey={() => openKeyEditor(dep.channel_id)}
                          onDelete={() => handleDeleteDeployment(dep.id)}
                        />
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
        <AddVirtualModelPanel
          collapsed={!showAddVM}
          onExpand={() => setShowAddVM(true)}
          name={newVMName}
          strategy={newVMStrategy}
          pool={newVMPool}
          onNameChange={setNewVMName}
          onStrategyChange={setNewVMStrategy}
          onPoolChange={setNewVMPool}
          onCancel={() => { setShowAddVM(false); setNewVMName(''); setNewVMPool(''); }}
          onSubmit={handleAddVirtualModel}
          saving={saving}
        />
      </div>

      {/* Edit base_url Modal */}
      <BaseUrlModal
        state={baseUrlModal}
        onChange={(partial) => setBaseUrlModal((prev) => (prev ? { ...prev, ...partial } : prev))}
        onClose={() => setBaseUrlModal(null)}
        onSave={saveBaseUrl}
      />

      {/* Edit key Modal */}
      <KeyModal
        state={keyModal}
        onChange={(partial) => setKeyModal((prev) => (prev ? { ...prev, ...partial } : prev))}
        onClose={() => setKeyModal(null)}
        onSave={saveKey}
      />
    </div>
  );
};

export default ModelEditor;
