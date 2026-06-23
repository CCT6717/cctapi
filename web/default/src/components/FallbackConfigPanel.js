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
  Modal,
  Table,
} from 'semantic-ui-react';
import { API } from '../helpers';
import './FallbackConfigPanel.css';

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
  const [draftRoutingVm, setDraftRoutingVm] = useState({}); // { [vmKey]: strategy }
  const [saveMessage, setSaveMessage] = useState(null);
  const [channels, setChannels] = useState([]);
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

  const handleRoutingModeChange = useCallback((vmKey, strategy) => {
    setDraftRoutingVm((prev) => ({ ...prev, [vmKey]: strategy }));
  }, []);

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
      // Edit user-editable UI fields: enabled / priority / weight, and the
      // 4 quota fields (quota_mode + 3 limits). Never touch strategy/pools
      // or other carry-over fields.
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
            if (draft.quota_mode !== undefined) {
              dep.quota_mode = draft.quota_mode;
            }
            if (draft.daily_limit_tokens !== undefined) {
              dep.daily_limit_tokens = Number(draft.daily_limit_tokens) || 0;
            }
            if (draft.soft_limit_ratio !== undefined) {
              // ponytail: Number.isFinite keeps 0 honest; || folds 0 to 0 too, but
              // isFinite also filters NaN/empty. Backend if<=0 restores default 0.95.
              const n = Number(draft.soft_limit_ratio);
              dep.soft_limit_ratio = Number.isFinite(n) ? n : 0;
            }
            if (draft.hard_limit_ratio !== undefined) {
              const n = Number(draft.hard_limit_ratio);
              dep.hard_limit_ratio = Number.isFinite(n) ? n : 0;
            }
            payload.deployments[id] = dep;
          }
        });
      }

      // Step 3.5: overlay routing strategy for VMs with a draft change
      // strategy is the source of truth (backend v2 contract)
      if (payload.virtual_models) {
        Object.keys(draftRoutingVm).forEach((vmKey) => {
          if (!payload.virtual_models[vmKey]) return;
          const target = draftRoutingVm[vmKey];
          if (target) {
            payload.virtual_models[vmKey].strategy = target;
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
        setDraftRoutingVm({});
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
  }, [draftDeployments, draftRoutingVm, loadConfig, loadDeploymentStatuses]);

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
                      const ownerNames = getDeploymentOwnerNames(
                        vmArray,
                        deploymentId
                      );
                      const ownerText = ownerNames.join(' / ');

                      return (
                        <div
                          className={`fallback-deployment-panel ${highlightDeployment === deploymentId ? 'fallback-highlight' : ''}`}
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
                                color='teal'
                              >
                                {`顺序 #${orderIndex + 1}`}
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
                                    <Table.Cell>配额模式</Table.Cell>
                                    <Table.Cell>
                                      <Dropdown
                                        selection
                                        compact
                                        options={[
                                          { key: 'paid', value: 'paid', text: '限额管理' },
                                          { key: 'free', value: 'free', text: '用完即换' },
                                        ]}
                                        value={draftDeployments[dep.id]?.quota_mode ?? dep.quota_mode ?? 'paid'}
                                        onChange={(_, { value }) => setDraftField(dep.id, 'quota_mode', value)}
                                      />
                                    </Table.Cell>
                                  </Table.Row>
                                  <Table.Row>
                                    <Table.Cell>每日 Token 限额</Table.Cell>
                                    <Table.Cell>
                                      <Input
                                        type='number'
                                        size='mini'
                                        style={{ maxWidth: 140 }}
                                        disabled={(() => {
                                          const mode = draftDeployments[dep.id]?.quota_mode ?? dep.quota_mode ?? 'paid';
                                          return mode === 'free';
                                        })()}
                                        placeholder='0 = 无限制'
                                        value={draftDeployments[dep.id]?.daily_limit_tokens ?? dep.daily_limit_tokens ?? 0}
                                        onChange={(_, { value }) => setDraftField(dep.id, 'daily_limit_tokens', value)}
                                      />
                                    </Table.Cell>
                                  </Table.Row>
                                  <Table.Row>
                                    <Table.Cell>软限比例</Table.Cell>
                                    <Table.Cell>
                                      <Input
                                        type='number'
                                        size='mini'
                                        step='0.01'
                                        min='0'
                                        max='1'
                                        style={{ maxWidth: 100 }}
                                        disabled={(() => {
                                          const mode = draftDeployments[dep.id]?.quota_mode ?? dep.quota_mode ?? 'paid';
                                          return mode === 'free';
                                        })()}
                                        placeholder='默认 0.95'
                                        value={draftDeployments[dep.id]?.soft_limit_ratio ?? dep.soft_limit_ratio ?? 0.95}
                                        onChange={(_, { value }) => setDraftField(dep.id, 'soft_limit_ratio', value)}
                                      />
                                    </Table.Cell>
                                  </Table.Row>
                                  <Table.Row>
                                    <Table.Cell>硬限比例</Table.Cell>
                                    <Table.Cell>
                                      <Input
                                        type='number'
                                        size='mini'
                                        step='0.01'
                                        min='0'
                                        max='1'
                                        style={{ maxWidth: 100 }}
                                        disabled={(() => {
                                          const mode = draftDeployments[dep.id]?.quota_mode ?? dep.quota_mode ?? 'paid';
                                          return mode === 'free';
                                        })()}
                                        placeholder='默认 1.0'
                                        value={draftDeployments[dep.id]?.hard_limit_ratio ?? dep.hard_limit_ratio ?? 1.0}
                                        onChange={(_, { value }) => setDraftField(dep.id, 'hard_limit_ratio', value)}
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
                                          color='blue'
                                          icon
                                          labelPosition='left'
                                          disabled={!dep.channel_id || saving}
                                          onClick={() => openBaseUrlEditor(dep.channel_id)}
                                          title={!dep.channel_id ? '该部署未绑定渠道' : '编辑该渠道的 base_url'}
                                        >
                                          <Icon name='linkify' />
                                          编辑 base_url
                                        </Button>
                                        <Button
                                          size='mini'
                                          icon
                                          labelPosition='left'
                                          disabled={!dep.channel_id || saving}
                                          onClick={() => openKeyEditor(dep.channel_id)}
                                          title={!dep.channel_id ? '该部署未绑定渠道' : '用新值覆盖该渠道的 key (原值不可查)'}
                                        >
                                          <Icon name='key' />
                                          编辑 key
                                        </Button>
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

                              {/* D3-b: 错误触发 fallback 规则（只读静态参考） */}
                              <details style={{ marginTop: 12 }}>
                                <summary style={{ cursor: 'pointer', fontSize: 13, color: '#475569', fontWeight: 600 }}>
                                  <Icon name='info circle' /> 错误触发 fallback 规则（只读参考）
                                </summary>
                                <Table compact celled size='small' style={{ marginTop: 8 }}>
                                  <Table.Header>
                                    <Table.Row>
                                      <Table.HeaderCell>错误类型</Table.HeaderCell>
                                      <Table.HeaderCell>触发切换</Table.HeaderCell>
                                      <Table.HeaderCell>部署状态</Table.HeaderCell>
                                    </Table.Row>
                                  </Table.Header>
                                  <Table.Body>
                                    <Table.Row>
                                      <Table.Cell>429 限速</Table.Cell>
                                      <Table.Cell><Label basic size='mini' color='green'>✓ 切换</Label></Table.Cell>
                                      <Table.Cell>冷却 60s~300s</Table.Cell>
                                    </Table.Row>
                                    <Table.Row>
                                      <Table.Cell>5xx 服务错误</Table.Cell>
                                      <Table.Cell><Label basic size='mini' color='green'>✓ 切换</Label></Table.Cell>
                                      <Table.Cell>冷却 60s~300s</Table.Cell>
                                    </Table.Row>
                                    <Table.Row>
                                      <Table.Cell>402 配额用尽</Table.Cell>
                                      <Table.Cell><Label basic size='mini' color='green'>✓ 切换</Label></Table.Cell>
                                      <Table.Cell>标记耗尽到当日末</Table.Cell>
                                    </Table.Row>
                                    <Table.Row>
                                      <Table.Cell>401/403/404</Table.Cell>
                                      <Table.Cell><Label basic size='mini' color='green'>✓ 切换</Label></Table.Cell>
                                      <Table.Cell>标记冷却 60s</Table.Cell>
                                    </Table.Row>
                                    <Table.Row>
                                      <Table.Cell>400 参数错误</Table.Cell>
                                      <Table.Cell><Label basic size='mini' color='grey'>✗ 不切换</Label></Table.Cell>
                                      <Table.Cell>直接返回错误</Table.Cell>
                                    </Table.Row>
                                  </Table.Body>
                                </Table>
                                <div style={{ fontSize: 12, color: '#94a3b8', marginTop: 6, fontStyle: 'italic' }}>
                                  注：流式响应已开始写入则不可切换（避免客户端收到半截响应）
                                </div>
                              </details>
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
                    { key: 'quality_first', text: '质量优先', value: 'quality_first' },
                    { key: 'cost_first', text: '成本优先', value: 'cost_first' },
                    { key: 'free_first', text: '免费优先', value: 'free_first' },
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

      {/* Edit base_url Modal */}
      <Modal
        open={!!baseUrlModal}
        onClose={() => !baseUrlModal?.saving && setBaseUrlModal(null)}
        size='small'
        closeOnEscape={!baseUrlModal?.saving}
        closeOnDimmerClick={!baseUrlModal?.saving}
      >
        <Modal.Header>
          编辑 base_url {baseUrlModal?.channelId ? `(渠道 #${baseUrlModal.channelId})` : ''}
        </Modal.Header>
        <Modal.Content>
          <Modal.Description>
            <p style={{ marginBottom: 12, color: '#475569' }}>
              修改该渠道的 <code>base_url</code>，保存后立即生效。
              <br />
              <span style={{ color: '#94a3b8', fontSize: 12 }}>
                末尾斜杠会被自动去除（统一格式）。
              </span>
            </p>
            <Input
              fluid
              placeholder='https://api.example.com/v1'
              value={baseUrlModal?.baseUrl || ''}
              onChange={(_, { value }) =>
                setBaseUrlModal((prev) => (prev ? { ...prev, baseUrl: value, error: '' } : prev))
              }
              disabled={baseUrlModal?.saving}
            />
            {baseUrlModal?.error && (
              <Message negative size='small' style={{ marginTop: 12 }}>
                <p>{baseUrlModal.error}</p>
              </Message>
            )}
          </Modal.Description>
        </Modal.Content>
        <Modal.Actions>
          <Button
            onClick={() => setBaseUrlModal(null)}
            disabled={baseUrlModal?.saving}
          >
            取消
          </Button>
          <Button
            color='blue'
            loading={baseUrlModal?.saving}
            disabled={baseUrlModal?.saving}
            onClick={saveBaseUrl}
          >
            <Icon name='check' />
            保存
          </Button>
        </Modal.Actions>
      </Modal>

      {/* Edit key Modal */}
      <Modal
        open={!!keyModal}
        onClose={() => !keyModal?.saving && setKeyModal(null)}
        size='tiny'
        closeOnEscape={!keyModal?.saving}
        closeOnDimmerClick={!keyModal?.saving}
      >
        <Modal.Header>
          编辑渠道 key {keyModal?.channelId ? `(渠道 #${keyModal.channelId})` : ''}
        </Modal.Header>
        <Modal.Content>
          <Modal.Description>
            <p style={{ marginBottom: 12, color: '#475569' }}>
              GET 接口不返回原 key，只能用新值覆盖。
            </p>
            <Input
              fluid
              type={keyModal?.showPlain ? 'text' : 'password'}
              placeholder='输入新 key'
              value={keyModal?.newKey || ''}
              onChange={(_, { value }) =>
                setKeyModal((prev) => (prev ? { ...prev, newKey: value, error: '' } : prev))
              }
              disabled={keyModal?.saving}
            />
            <div style={{ marginTop: 10 }}>
              <Checkbox
                label='显示明文'
                checked={!!keyModal?.showPlain}
                disabled={keyModal?.saving}
                onChange={(_, { checked }) =>
                  setKeyModal((prev) => (prev ? { ...prev, showPlain: checked } : prev))
                }
              />
            </div>
            {keyModal?.error && (
              <Message negative size='small' style={{ marginTop: 12 }}>
                <p>{keyModal.error}</p>
              </Message>
            )}
          </Modal.Description>
        </Modal.Content>
        <Modal.Actions>
          <Button
            onClick={() => setKeyModal(null)}
            disabled={keyModal?.saving}
          >
            取消
          </Button>
          <Button
            color='blue'
            loading={keyModal?.saving}
            disabled={keyModal?.saving || !keyModal?.newKey}
            onClick={saveKey}
          >
            <Icon name='check' />
            保存
          </Button>
        </Modal.Actions>
      </Modal>
    </div>
  );
};

export default ModelEditor;
