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
import { useFallbackSave } from './hooks/useFallbackSave';
import {
  isSeparatorKey,
  isFreeDeployment,
  slugModelName,
  computeInitialMode,
  getVirtualModelDeploymentIds,
  applyVmModeSelection,
  getDeploymentStatusMeta,
  getDeploymentOwnerNames,
} from './utils/deploymentMeta';
import './FallbackConfigPanel.css';

const HIDDEN_VMS = [];

const ModelEditor = ({ highlightDeployment }) => {
  const { config, allDeployments, loading, error, loadConfig, deploymentMode, setDeploymentMode } = useGatewayConfig();
  const { deploymentStatuses, loadDeploymentStatuses } = useDeploymentStatuses();
  const { channels, loadChannels } = useChannels('manual');

  const [expandedVirtualModels, setExpandedVirtualModels] = useState({});
  const [expandedDeployments, setExpandedDeployments] = useState({});
  const { execute, saving, saveMessage, setSaveMessage } = useFallbackSave({ loadConfig, loadDeploymentStatuses });
  const [draftDeployments, setDraftDeployments] = useState({});
  const [draftRoutingVm, setDraftRoutingVm] = useState({});
  const [selectorState, setSelectorState] = useState({});
  const [healthTesting, setHealthTesting] = useState({});
  const [healthResults, setHealthResults] = useState({});
  const [showAddVM, setShowAddVM] = useState(false);
  const [newVMName, setNewVMName] = useState('');
  const [newVMStrategy, setNewVMStrategy] = useState('quality_first');
  const [newVMPool, setNewVMPool] = useState('');
  const [baseUrlModal, setBaseUrlModal] = useState(null);
  const [keyModal, setKeyModal] = useState(null);

  const fullDeploymentMap = useMemo(() => allDeployments || config?.deployments || {}, [allDeployments, config]);

  const visibleDeploymentIds = useMemo(() => {
    if (!config?.deployments) return [];
    return Object.keys(config.deployments).filter((id) => !isSeparatorKey(id));
  }, [config]);

  const vmArray = useMemo(() => {
    if (!config?.virtual_models) return [];
    return Object.keys(config.virtual_models)
      .filter((name) => !HIDDEN_VMS.includes(name))
      .map((name) => {
        const vm = config.virtual_models[name];
        if (!vm) return null;
        const fallbackOrder = getVirtualModelDeploymentIds(vm, config.deployments || {});
        return {
          name,
          ...vm,
          fallback_order: fallbackOrder,
        };
      })
      .filter(Boolean);
  }, [config]);

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

  const channelNameMap = useMemo(() => {
    const map = {};
    (channels || []).forEach((ch) => { map[ch.id] = ch.name; });
    return map;
  }, [channels]);

  const channelDataMap = useMemo(() => {
    const map = {};
    (channels || []).forEach((ch) => { map[ch.id] = ch; });
    return map;
  }, [channels]);

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

  const existingPairsByVm = useMemo(() => {
    const map = {};
    if (!config?.deployments || !config?.virtual_models) return map;
    Object.entries(config.virtual_models).forEach(([vmKey, vm]) => {
      getVirtualModelDeploymentIds(vm, config.deployments).forEach((id) => {
        const dep = config.deployments[id];
        if (isSeparatorKey(id)) return;
        if (isFreeDeployment(id, dep)) return;
        if (dep?.channel_id && dep?.real_model) {
          if (!map[vmKey]) map[vmKey] = new Set();
          map[vmKey].add(`${dep.channel_id}:${dep.real_model}`);
        }
      });
    });
    return map;
  }, [config]);

  const manualChannels = channels;

  const setDraftField = (depId, field, value) => {
    setDraftDeployments((prev) => ({
      ...prev,
      [depId]: {
        ...prev[depId],
        [field]: value,
      },
    }));
  };

  const deploymentOwnerVm = useMemo(() => {
    const map = {};
    if (config?.virtual_models && config?.deployments) {
      Object.entries(config.virtual_models).forEach(([vmKey, vm]) => {
        getVirtualModelDeploymentIds(vm, config.deployments).forEach((depId) => {
          map[depId] = Array.isArray(map[depId]) ? map[depId] : [];
          if (!map[depId].includes(vmKey)) map[depId].push(vmKey);
        });
      });
    }
    return map;
  }, [config]);

  const handleModeChange = useCallback((depId, mode, vmKey) => {
    const currentDep = config?.deployments?.[depId];
    if (isFreeDeployment(depId, currentDep)) {
      setSaveMessage({ type: 'error', text: '免费部署不可在模型编辑器中修改模式，请到「免费模型池」面板编辑' });
      return;
    }
    setDeploymentMode((prev) => {
      return applyVmModeSelection({
        previousModes: prev,
        depId,
        mode,
        vmKey,
        data: config,
      });
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
  }, [config, setDeploymentMode, setDraftDeployments, setSaveMessage]);

  const handleSave = useCallback(async () => {
    await execute(
      (fresh) => buildSavePayload(fresh, { draftDeployments, draftRoutingVm, deploymentMode, deploymentOwnerVm }),
      {
        successMsg: '保存成功',
        onSaved: () => { setDraftDeployments({}); setDraftRoutingVm({}); },
      }
    );
  }, [execute, draftDeployments, draftRoutingVm, deploymentMode, deploymentOwnerVm]);

  const handleAddDeployment = useCallback(async (channelId, model, pool, vmKey) => {
    await execute(
      (fresh) => {
        const payload = JSON.parse(JSON.stringify(fresh));
        if (!payload.deployments) payload.deployments = {};
        const vmPools = new Set(config?.virtual_models?.[vmKey]?.pools || []);
        for (const [, dep] of Object.entries(payload.deployments)) {
          if (dep?.channel_id === channelId && dep?.real_model === model && vmPools.has(dep.pool)) {
            setSaveMessage({ type: 'error', text: `该渠道已有模型 ${model}，不可重复添加` });
            return null;
          }
        }
        let baseId = `manual-${channelId}-${slugModelName(model)}`;
        if (baseId.startsWith('free:') || baseId.startsWith('---')) {
          baseId = `m-${channelId}-${slugModelName(model)}`;
        }
        let newId = baseId;
        let suffix = 1;
        while (payload.deployments[newId]) { newId = `${baseId}-${suffix}`; suffix++; }
        payload.deployments[newId] = { enabled: true, channel_id: channelId, real_model: model, pool, priority: 0, weight: 100 };
        const vm = payload.virtual_models?.[vmKey];
        if (vm) {
          let order = Array.isArray(vm.fallback_order) ? [...vm.fallback_order] : [];
          if (order.length === 0) {
            const pools = vm.pools || [];
            order = Object.entries(payload.deployments || {})
              .filter(([id, d]) => !id.startsWith('---') && d && pools.includes(d.pool) && id !== newId)
              .map(([id]) => id);
          }
          order.push(newId);
          vm.fallback_order = order;
        }
        return payload;
      },
      {
        successMsg: '已添加部署',
        onSaved: () => setSelectorState((prev) => ({ ...prev, [vmKey]: null })),
      }
    );
  }, [config, execute, setSaveMessage]);

  const handleDeleteDeployment = useCallback(async (deploymentId, fromVmKey) => {
    if (!deploymentId || !fromVmKey) return;
    const currentDep = config?.deployments?.[deploymentId];
    if (isFreeDeployment(deploymentId, currentDep)) {
      setSaveMessage({ type: 'error', text: '免费部署不可在模型编辑器中删除' });
      return;
    }
    await execute(
      (fresh) => {
        if (!fresh.deployments?.[deploymentId]) {
          setSaveMessage({ type: 'error', text: `部署 ${deploymentId} 不存在于最新配置中` });
          return null;
        }
        const payload = JSON.parse(JSON.stringify(fresh));
        const vm = payload.virtual_models?.[fromVmKey];
        if (!vm) {
          setSaveMessage({ type: 'error', text: `虚拟模型 ${fromVmKey} 不存在` });
          return null;
        }
        let order = Array.isArray(vm.fallback_order) ? [...vm.fallback_order] : [];
        if (order.length === 0) {
          const pools = vm.pools || [];
          order = Object.entries(payload.deployments || {})
            .filter(([id, d]) => !id.startsWith('---') && d && pools.includes(d.pool))
            .map(([id]) => id);
        }
        vm.fallback_order = order.filter((id) => id !== deploymentId);
        return payload;
      },
      {
        successMsg: `已从 ${fromVmKey} 移除部署`,
        onSaved: () => setDraftDeployments((prev) => {
          const next = { ...prev };
          delete next[deploymentId];
          return next;
        }),
      }
    );
  }, [config, execute, setSaveMessage]);

  const handleAddVirtualModel = useCallback(async () => {
    const name = newVMName.trim();
    if (!name) { setSaveMessage({ type: 'error', text: '虚拟模型名称不能为空' }); return; }
    if (name.startsWith('cct/')) { setSaveMessage({ type: 'error', text: 'cct/ 前缀保留给系统虚拟模型，请用其他名称' }); return; }
    const pool = newVMPool.trim() || 'default';
    await execute(
      (fresh) => {
        if (fresh.virtual_models?.[name]) {
          setSaveMessage({ type: 'error', text: `虚拟模型 ${name} 已存在` });
          return null;
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
        return payload;
      },
      {
        successMsg: `已添加虚拟模型 ${name}`,
        onSaved: () => { setNewVMName(''); setNewVMStrategy('quality_first'); setNewVMPool(''); setShowAddVM(false); },
      }
    );
  }, [newVMName, newVMStrategy, newVMPool, execute, setSaveMessage]);

  const handleDeleteVirtualModel = useCallback(async (vmName) => {
    if (!vmName || HIDDEN_VMS.includes(vmName)) return;
    if (vmName.startsWith('cct/')) { setSaveMessage({ type: 'error', text: '系统虚拟模型不可删除' }); return; }
    if (!window.confirm(`确定要删除虚拟模型 ${vmName} 吗？`)) return;
    await execute(
      (fresh) => {
        if (!fresh.virtual_models?.[vmName]) {
          setSaveMessage({ type: 'error', text: `虚拟模型 ${vmName} 不存在于最新配置中` });
          return null;
        }
        const payload = JSON.parse(JSON.stringify(fresh));
        delete payload.virtual_models[vmName];
        return payload;
      },
      { successMsg: `已删除虚拟模型 ${vmName}` }
    );
  }, [execute, setSaveMessage]);

  const saveBaseUrl = useCallback(async () => {
    if (!baseUrlModal?.channelId) return;
    setBaseUrlModal((prev) => ({ ...prev, saving: true, error: '' }));
    try {
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
  }, [baseUrlModal, setSaveMessage]);

  const saveKey = useCallback(async () => {
    if (!keyModal?.channelId) return;
    if (!keyModal.newKey) {
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
  }, [keyModal, setSaveMessage]);

  const saveChannelField = useCallback(async (channelId, fields, onDone) => {
    setSaveMessage(null);
    try {
      const payload = { id: channelId, ...fields };
      const res = await API.put('/api/channel/', payload);
      if (res.data?.success) {
        setSaveMessage({ type: 'success', text: `渠道 #${channelId} 已更新` });
        await loadChannels();
        if (onDone) onDone();
      } else {
        setSaveMessage({ type: 'error', text: res.data?.message || '保存失败' });
      }
    } catch (e) {
      setSaveMessage({ type: 'error', text: e.message || '保存异常' });
    }
  }, [loadChannels, setSaveMessage]);

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
          const fixedDepId = (vm.fallback_order || []).find((depId) => deploymentMode[depId] === 'fixed')
            || (vm.fallback_order || []).find(
              (depId) => deploymentMode[depId] === undefined && computeInitialMode(config, depId, vmKey) === 'fixed'
            );
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
                  </div>
                  <div className='fallback-virtual-meta'>
                    {modelCount} 个真实模型
                    {!!fixedDepId && (
                      <span style={{ marginLeft: 8 }}> · 固定模式</span>
                    )}
                  </div>
                </div>
                <div className='fallback-virtual-summary-actions'>
                  <Label basic color={vm.enabled ? 'green' : 'grey'}>
                    {vm.enabled ? '启用' : '停用'}
                  </Label>
                  <Button
                    size='small'
                    basic
                    color='blue'
                    className='fallback-btn-test-all'
                    disabled={saving}
                    onClick={() => {
                      (vm.fallback_order || []).forEach((id) => {
                        const dep = config?.deployments?.[id] || fullDeploymentMap?.[id];
                        if (isFreeDeployment(id, dep)) return;
                        handleHealthCheck(id);
                      });
                    }}
                  >
                    <Icon name='heartbeat' />
                    测试全部
                  </Button>
                  {!vm.name.startsWith('cct/') && (
                    <Button
                      size='mini'
                      negative
                      loading={saving}
                      onClick={() => handleDeleteVirtualModel(vm.name)}
                    >
                      <Icon name='trash' />
                      删除
                    </Button>
                  )}
                </div>
              </div>

              {vmExpanded && (
                <div className='fallback-virtual-body'>
                  <div className='fallback-add-model'>
                    <div className='fallback-add-model-header'>
                      <Icon name='plus circle' />
                      <span>添加真实模型</span>
                    </div>
                    <div className='fallback-add-model-selector'>
                      {manualChannels.length === 0 ? (
                        <div className='fallback-add-model-empty'>
                          <Icon name='info circle' />
                          暂无可用渠道，请先在渠道管理中添加渠道
                        </div>
                      ) : (
                        <Dropdown
                          placeholder='搜索渠道或模型名称...'
                          fluid
                          search
                          selection
                          value={selectorState[vmKey]?.value || ''}
                          options={(() => {
                            const opts = [];
                            const vmPairs = existingPairsByVm[vmKey] || new Set();
                            manualChannels.forEach((ch) => {
                              ch.models.forEach((model) => {
                                const pairKey = `${ch.id}:${model}`;
                                const exists = vmPairs.has(pairKey);
                                opts.push({
                                  key: pairKey,
                                  value: pairKey,
                                  disabled: exists,
                                  className: exists ? 'fallback-model-item disabled' : 'fallback-model-item',
                                  content: (
                                    <div className='fallback-model-item-content'>
                                      <span className='fallback-model-channel'>{ch.name}</span>
                                      <span className='fallback-model-sep'>/</span>
                                      <span className='fallback-model-name'>{model}</span>
                                      {exists && <span className='fallback-model-badge'>已添加</span>}
                                    </div>
                                  ),
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
                      )}
                    </div>

                    {selectorState[vmKey] && (() => {
                      const sel = selectorState[vmKey];
                      return (
                        <div className='fallback-add-model-preview'>
                          <div className='fallback-add-model-preview-content'>
                            <div className='fallback-add-model-preview-field'>
                              <strong>渠道:</strong>
                              <span>#{sel.channelId}</span>
                            </div>
                            <div className='fallback-add-model-preview-field'>
                              <strong>模型:</strong>
                              <span>{sel.model}</span>
                            </div>
                            <div className='fallback-add-model-preview-actions'>
                              <Button
                                size='small'
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
                              >
                                <Icon name='plus' />
                                添加到 {vm.name}
                              </Button>
                              <Button
                                size='small'
                                basic
                                onClick={() => setSelectorState((prev) => ({ ...prev, [vmKey]: null }))}
                              >
                                取消
                              </Button>
                            </div>
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
                      const rowMode = deploymentMode[dep.id] || computeInitialMode(config, dep.id, vmKey);
                      const currentMode = rowMode;

                      return (
                        <DeploymentRow
                          key={deploymentKey}
                          dep={dep}
                          channelName={channelNameMap[dep.channel_id] || ''}
                          channelData={channelDataMap[dep.channel_id] || null}
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
                          onSave={(chId, fields, onDone) => saveChannelField(chId, fields, onDone)}
                          onDelete={() => handleDeleteDeployment(dep.id, vmKey)}
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

      <BaseUrlModal
        state={baseUrlModal}
        onChange={(partial) => setBaseUrlModal((prev) => (prev ? { ...prev, ...partial } : prev))}
        onClose={() => setBaseUrlModal(null)}
        onSave={saveBaseUrl}
      />

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
