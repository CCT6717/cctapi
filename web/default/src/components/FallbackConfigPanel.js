import React, { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Button,
  Checkbox,
  Divider,
  Form,
  Header,
  Icon,
  Input,
  Label,
  Loader,
  Message,
  Modal,
  Popup,
} from 'semantic-ui-react';
import { API, showError, showInfo, showSuccess } from '../helpers';
import './FallbackConfigPanel.css';

const OPENAI_COMPATIBLE_CHANNEL_TYPE = 50;
const ROUTING_MODE_WEIGHTED = 'weighted';
const ROUTING_MODE_SEQUENTIAL = 'sequential';
const ROUTING_MODE_FIXED = 'fixed';
const ROUTING_MODE_META = {
  [ROUTING_MODE_WEIGHTED]: {
    title: '按权重',
    detail: '健康 deployment 按 weight 比例分流，适合主力和备用同时消耗。',
    icon: 'random',
    color: 'blue',
  },
  [ROUTING_MODE_SEQUENTIAL]: {
    title: '按顺序',
    detail: '严格按列表顺序尝试，前一个不可用或额度到线后再切下一个。',
    icon: 'sort amount down',
    color: 'teal',
  },
  [ROUTING_MODE_FIXED]: {
    title: '固定模型',
    detail: '始终路由到指定真实模型，适合手动锁定主力部署。',
    icon: 'bullseye',
    color: 'purple',
  },
};
const ROUTING_MODE_OPTIONS = [
  ROUTING_MODE_WEIGHTED,
  ROUTING_MODE_SEQUENTIAL,
  ROUTING_MODE_FIXED,
];

const normalizeBaseUrlForChannelType = (channelType, baseUrl) => {
  const normalized = String(baseUrl || '').trim().replace(/\/+$/, '');
  if (Number(channelType) === 40) {
    return normalized.replace(/\/api\/v3$/i, '');
  }
  if (Number(channelType) === 20) {
    return normalized.replace(/\/v1$/i, '');
  }
  return normalized;
};

const defaultChannel = () => ({
  id: 0,
  name: '',
  type: OPENAI_COMPATIBLE_CHANNEL_TYPE,
  base_url: '',
  key: '',
  models: '',
  model_list: [],
  status: 1,
});

const defaultDeployment = (id, priority) => ({
  id,
  enabled: true,
  channel_id: 0,
  real_model: '',
  priority,
  weight: 100,
  max_concurrent_requests: 3,
  daily_limit_tokens: 1000000,
  quota_mode: 'controlled',
  soft_limit_ratio: 0.9,
  hard_limit_ratio: 0.98,
  max_context: 0,
  min_context: 0,
  channel: defaultChannel(),
});

const getDeploymentOwnerNames = (virtualModels, deploymentId) =>
  (virtualModels || [])
    .filter((vm) => (vm.fallback_order || []).includes(deploymentId))
    .map((vm) => vm.name || '未命名虚拟模型');

const normalizeConfig = (data) => ({
  enabled: data?.enabled ?? true,
  virtual_models: Array.isArray(data?.virtual_models)
    ? data.virtual_models.map((vm) => ({
        name: vm.name || '',
        enabled: vm.enabled ?? true,
        description: vm.description || '',
        routing_mode: vm.routing_mode || ROUTING_MODE_WEIGHTED,
        fixed_deployment: vm.fixed_deployment || '',
        fallback_order: Array.isArray(vm.fallback_order)
          ? vm.fallback_order
          : [],
      }))
    : [],
  deployments: Array.isArray(data?.deployments)
    ? data.deployments.map((dep) => ({
        ...dep,
        enabled: dep.enabled ?? true,
        channel_id: dep.channel_id || dep.channel?.id || 0,
        weight: dep.weight || 100,
        max_concurrent_requests: dep.max_concurrent_requests ?? 0,
        daily_limit_tokens: dep.daily_limit_tokens || 0,
        quota_mode: dep.quota_mode || 'controlled',
        soft_limit_ratio: dep.soft_limit_ratio || 0.9,
        hard_limit_ratio: dep.hard_limit_ratio || 0.98,
        channel: {
          ...defaultChannel(),
          ...(dep.channel || {}),
          id: dep.channel?.id || dep.channel_id || 0,
        },
      }))
    : [],
  channels: Array.isArray(data?.channels)
    ? data.channels.map((channel) => ({
        ...defaultChannel(),
        ...channel,
        id: channel.id || 0,
        model_list: Array.isArray(channel.model_list)
          ? channel.model_list
          : String(channel.models || '')
              .split(',')
              .map((model) => model.trim())
              .filter(Boolean),
      }))
    : [],
  alert: data?.alert || {},
  smart_sort: data?.smart_sort || {},
});

const makeDeploymentId = (vmName, deployments) => {
  const prefix = (vmName || 'virtual')
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '') || 'virtual';
  let index = deployments.length + 1;
  let id = `${prefix}-model-${index}`;
  const used = new Set(deployments.map((dep) => dep.id));
  while (used.has(id)) {
    index += 1;
    id = `${prefix}-model-${index}`;
  }
  return id;
};

const FallbackConfigPanel = ({ highlightDeployment }) => {
  const [config, setConfig] = useState(null);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [savingVirtualModel, setSavingVirtualModel] = useState('');
  const [batchLoading, setBatchLoading] = useState('');
  const [testing, setTesting] = useState(false);
  const [testingSingle, setTestingSingle] = useState(null);
  const [testResults, setTestResults] = useState({});
  const [deploymentStatuses, setDeploymentStatuses] = useState({});
  const [actingDeployment, setActingDeployment] = useState('');
  const [expandedVirtualModels, setExpandedVirtualModels] = useState({});
  const [expandedDeployments, setExpandedDeployments] = useState({});
  const [orderingVirtualModels, setOrderingVirtualModels] = useState({});
  const [visibleKeys, setVisibleKeys] = useState({});
  const [diffModalOpen, setDiffModalOpen] = useState(false);
  const [diffContent, setDiffContent] = useState([]);

  const deploymentsById = useMemo(() => {
    const map = {};
    (config?.deployments || []).forEach((dep) => {
      map[dep.id] = dep;
    });
    return map;
  }, [config]);

  const channelTemplateOptions = useMemo(() => {
    const options = [];
    (config?.channels || []).forEach((channel) => {
      const channelId = Number(channel.id || 0);
      if (!channelId) {
        return;
      }
      const channelName = channel.name || `channel-${channelId}`;
      const models =
        Array.isArray(channel.model_list) && channel.model_list.length > 0
          ? channel.model_list
          : [''];
      models.forEach((modelName, modelIndex) => {
        const model = String(modelName || '').trim();
        options.push({
          key: `${channelId}:${model || modelIndex}`,
          value: `${channelId}::${model}`,
          text: `${channelName}${model ? ` / ${model}` : ''}`,
          description: channel.base_url || '',
        });
      });
    });
    return options.sort((a, b) => a.text.localeCompare(b.text, 'zh-Hans-CN'));
  }, [config]);

  const loadDeploymentStatuses = useCallback(async (silent = false) => {
    try {
      const res = await API.get('/api/fallback/alert/status');
      const statusList = Array.isArray(res.data?.status) ? res.data.status : [];
      const statusMap = {};
      statusList.forEach((status) => {
        if (status?.deployment_id) {
          statusMap[status.deployment_id] = status;
        }
      });
      setDeploymentStatuses(statusMap);
    } catch (error) {
      if (!silent) {
        showError(error.message || '加载部署状态失败');
      }
    }
  }, []);

  const loadConfig = useCallback(async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/fallback/editor/config');
      const { success, message, data } = res.data;
      if (success) {
        const normalizedConfig = normalizeConfig(data);
        setConfig(normalizedConfig);
        setExpandedVirtualModels({});
        setExpandedDeployments({});
        setOrderingVirtualModels({});
        setVisibleKeys({});
        setTestResults({});
        await loadDeploymentStatuses(true);
      } else {
        showError(message || '加载虚拟模型配置失败');
      }
    } catch (error) {
      showError(error.message || '加载虚拟模型配置失败');
    } finally {
      setLoading(false);
    }
  }, [loadDeploymentStatuses]);

  useEffect(() => {
    loadConfig().then();
  }, [loadConfig]);

  const setVirtualModel = (index, field, value) => {
    setConfig((oldConfig) => {
      const virtualModels = [...oldConfig.virtual_models];
      virtualModels[index] = { ...virtualModels[index], [field]: value };
      return { ...oldConfig, virtual_models: virtualModels };
    });
  };

  const setVirtualModelRoutingMode = (index, key, mode) => {
    setConfig((oldConfig) => {
      const virtualModels = [...oldConfig.virtual_models];
      const vm = virtualModels[index];
      const fallbackOrder = Array.isArray(vm.fallback_order)
        ? vm.fallback_order
        : [];
      virtualModels[index] = {
        ...vm,
        routing_mode: mode,
        fixed_deployment:
          mode === ROUTING_MODE_FIXED
            ? vm.fixed_deployment || fallbackOrder[0] || ''
            : vm.fixed_deployment || '',
      };
      return { ...oldConfig, virtual_models: virtualModels };
    });
    if (mode !== ROUTING_MODE_SEQUENTIAL) {
      setOrderingVirtualModels((oldState) => ({
        ...oldState,
        [key]: false,
      }));
    }
  };

  const setVirtualModelFixedDeployment = (vmIndex, deploymentId) => {
    setConfig((oldConfig) => {
      const virtualModels = [...oldConfig.virtual_models];
      const vm = virtualModels[vmIndex];
      virtualModels[vmIndex] = {
        ...vm,
        routing_mode: ROUTING_MODE_FIXED,
        fixed_deployment: deploymentId,
      };
      return { ...oldConfig, virtual_models: virtualModels };
    });
  };

  const toggleOrderEditor = (key) => {
    setOrderingVirtualModels((oldState) => ({
      ...oldState,
      [key]: !oldState[key],
    }));
  };

  const moveDeploymentInVirtualModel = (vmIndex, deploymentId, direction) => {
    setConfig((oldConfig) => {
      const virtualModels = [...oldConfig.virtual_models];
      const vm = virtualModels[vmIndex];
      const order = [...(vm.fallback_order || [])];
      const currentIndex = order.indexOf(deploymentId);
      const nextIndex = currentIndex + direction;
      if (
        currentIndex < 0 ||
        nextIndex < 0 ||
        nextIndex >= order.length
      ) {
        return oldConfig;
      }
      [order[currentIndex], order[nextIndex]] = [
        order[nextIndex],
        order[currentIndex],
      ];
      virtualModels[vmIndex] = { ...vm, fallback_order: order };
      return { ...oldConfig, virtual_models: virtualModels };
    });
  };

  const setDeployment = (id, field, value) => {
    setConfig((oldConfig) => ({
      ...oldConfig,
      deployments: oldConfig.deployments.map((dep) =>
        dep.id === id ? { ...dep, [field]: value } : dep
      ),
    }));
  };

  const applyDeploymentTemplate = (targetId, templateValue) => {
    setConfig((oldConfig) => {
      const [channelIdText, ...modelParts] = String(templateValue || '').split('::');
      const channelId = Number(channelIdText || 0);
      const model = modelParts.join('::');
      const channel = (oldConfig.channels || []).find(
        (item) => Number(item.id || 0) === channelId
      );
      if (!channel) {
        return oldConfig;
      }

      const templateChannel = {
        ...defaultChannel(),
        ...channel,
        id: channelId,
      };

      return {
        ...oldConfig,
        deployments: oldConfig.deployments.map((dep) =>
          dep.id === targetId
            ? {
                ...dep,
                channel_id: channelId,
                real_model: model || dep.real_model,
                channel: { ...templateChannel },
              }
            : dep
        ),
      };
    });
  };

  const setDeploymentQuotaMode = (id, value) => {
    setConfig((oldConfig) => ({
      ...oldConfig,
      deployments: oldConfig.deployments.map((dep) => {
        if (dep.id !== id) {
          return dep;
        }
        if (value === 'free') {
          return {
            ...dep,
            quota_mode: value,
            daily_limit_tokens: 0,
          };
        }
        return {
          ...dep,
          quota_mode: value,
          daily_limit_tokens:
            Number(dep.daily_limit_tokens) > 0
              ? dep.daily_limit_tokens
              : defaultDeployment(dep.id, dep.priority).daily_limit_tokens,
        };
      }),
    }));
  };

  const setDeploymentChannel = (id, field, value) => {
    setConfig((oldConfig) => ({
      ...oldConfig,
      deployments: oldConfig.deployments.map((dep) => {
        const target = oldConfig.deployments.find((item) => item.id === id);
        if (!target) {
          return dep;
        }

        const targetChannelId = Number(target.channel_id || target.channel?.id || 0);
        const depChannelId = Number(dep.channel_id || dep.channel?.id || 0);
        const shouldSync =
          dep.id === id || (targetChannelId > 0 && depChannelId === targetChannelId);

        if (!shouldSync) {
          return dep;
        }

        return {
          ...dep,
          channel_id: depChannelId || targetChannelId,
          channel: {
            ...defaultChannel(),
            ...dep.channel,
            [field]: value,
            id: depChannelId || targetChannelId || 0,
          },
        };
      }),
    }));
  };

  const getVirtualModelKey = (vm, index) => `${index}:${vm.name || 'virtual'}`;

  const getDeploymentKey = (vmIndex, deploymentId) =>
    `${vmIndex}:${deploymentId}`;

  const toggleVirtualModel = (key) => {
    setExpandedVirtualModels((oldState) => ({
      ...oldState,
      [key]: !oldState[key],
    }));
  };

  const toggleDeployment = (key) => {
    setExpandedDeployments((oldState) => ({
      ...oldState,
      [key]: !oldState[key],
    }));
  };

  const toggleKeyVisible = (deploymentId) => {
    setVisibleKeys((oldState) => ({
      ...oldState,
      [deploymentId]: !oldState[deploymentId],
    }));
  };

  const addVirtualModel = () => {
    setConfig((oldConfig) => {
      const index = oldConfig.virtual_models.length + 1;
      return {
        ...oldConfig,
        virtual_models: [
          ...oldConfig.virtual_models,
          {
            name: `virtual/auto-${index}`,
            enabled: true,
            description: '',
            routing_mode: ROUTING_MODE_WEIGHTED,
            fixed_deployment: '',
            fallback_order: [],
          },
        ],
      };
    });
  };

  const removeVirtualModel = (index) => {
    setConfig((oldConfig) => {
      const removed = oldConfig.virtual_models[index];
      const virtualModels = oldConfig.virtual_models.filter((_, i) => i !== index);
      const stillUsed = new Set(
        virtualModels.flatMap((vm) => vm.fallback_order || [])
      );
      const deployments = oldConfig.deployments.filter(
        (dep) => !removed.fallback_order.includes(dep.id) || stillUsed.has(dep.id)
      );
      return { ...oldConfig, virtual_models: virtualModels, deployments };
    });
  };

  const copyVirtualModel = (vmIndex) => {
    setConfig((oldConfig) => {
      const source = oldConfig.virtual_models[vmIndex];
      const newName = `${source.name}-copy`;
      const newIdPrefix = `${source.name}-copy`.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '') || 'copy';
      const depIdMap = {};

      const newDeployments = (source.fallback_order || []).map((depId) => {
        const sourceDep = oldConfig.deployments.find(d => d.id === depId);
        if (!sourceDep) return null;
        const newId = `${newIdPrefix}-${depId}`;
        depIdMap[depId] = newId;
        return {
          ...JSON.parse(JSON.stringify(sourceDep)),
          id: newId,
          channel_id: 0,
          channel: { ...sourceDep.channel, id: 0 },
        };
      }).filter(Boolean);

      const newVm = {
        ...source,
        name: newName,
        description: source.description ? `${source.description} (副本)` : '(副本)',
        fixed_deployment: source.fixed_deployment
          ? depIdMap[source.fixed_deployment] || ''
          : '',
        fallback_order: (source.fallback_order || []).map((depId) => depIdMap[depId] || `${newIdPrefix}-${depId}`),
      };

      return {
        ...oldConfig,
        virtual_models: [...oldConfig.virtual_models, newVm],
        deployments: [...oldConfig.deployments, ...newDeployments],
      };
    });
  };

  const addDeploymentToVirtualModel = (vmIndex) => {
    setConfig((oldConfig) => {
      const virtualModels = [...oldConfig.virtual_models];
      const vm = virtualModels[vmIndex];
      const id = makeDeploymentId(vm.name, oldConfig.deployments);
      const nextPriority =
        oldConfig.deployments.reduce(
          (max, dep) => Math.max(max, Number(dep.priority) || 0),
          0
        ) + 1;
      virtualModels[vmIndex] = {
        ...vm,
        fallback_order: [...vm.fallback_order, id],
        fixed_deployment:
          vm.routing_mode === ROUTING_MODE_FIXED && !vm.fixed_deployment
            ? id
            : vm.fixed_deployment || '',
      };
      return {
        ...oldConfig,
        virtual_models: virtualModels,
        deployments: [...oldConfig.deployments, defaultDeployment(id, nextPriority)],
      };
    });
  };

  const removeDeploymentFromVirtualModel = (vmIndex, deploymentId) => {
    setConfig((oldConfig) => {
      const virtualModels = oldConfig.virtual_models.map((vm, index) =>
        index === vmIndex
          ? {
              ...vm,
              fixed_deployment:
                vm.fixed_deployment === deploymentId
                  ? ''
                  : vm.fixed_deployment || '',
              fallback_order: vm.fallback_order.filter((id) => id !== deploymentId),
            }
          : vm
      );
      const stillUsed = virtualModels.some((vm) =>
        vm.fallback_order.includes(deploymentId)
      );
      return {
        ...oldConfig,
        virtual_models: virtualModels,
        deployments: stillUsed
          ? oldConfig.deployments
          : oldConfig.deployments.filter((dep) => dep.id !== deploymentId),
      };
    });
  };

  const getConfigValidationErrors = (nextConfig) => {
    const errors = [];
    const virtualModels = Array.isArray(nextConfig?.virtual_models)
      ? nextConfig.virtual_models
      : [];
    const deployments = Array.isArray(nextConfig?.deployments)
      ? nextConfig.deployments
      : [];

    if (virtualModels.length === 0) {
      errors.push('至少需要配置一个虚拟模型');
    }

    const deploymentIds = new Set();
    const enabledDeploymentIds = new Set();
    deployments.forEach((dep, index) => {
      const id = String(dep.id || '').trim();
      const label = id || `第 ${index + 1} 个真实模型`;
      const realModel = String(dep.real_model || '').trim();
      const channelName = String(dep.channel?.name || '').trim();
      const baseUrl = String(dep.channel?.base_url || '').trim();
      const key = String(dep.channel?.key_masked || '').trim();
      const weight = Number(dep.weight);
      const priority = Number(dep.priority);
      const dailyLimit = Number(dep.daily_limit_tokens || 0);
      const softLimitRatio = Number(dep.soft_limit_ratio || 0);
      const hardLimitRatio = Number(dep.hard_limit_ratio || 0);

      if (!id) {
        errors.push(`真实模型 #${index + 1} 的 deployment ID 不能为空`);
      } else if (deploymentIds.has(id)) {
        errors.push(`deployment ID 重复：${id}`);
      }
      if (id) {
        deploymentIds.add(id);
      }
      if (dep.enabled !== false && id) {
        enabledDeploymentIds.add(id);
      }
      if (!realModel) {
        errors.push(`${label} 的真实模型名不能为空`);
      }
      if (dep.enabled !== false) {
        if (!baseUrl) {
          errors.push(`${label} 的接口地址不能为空`);
        }
        const normalizedBaseUrl = normalizeBaseUrlForChannelType(dep.channel?.type, baseUrl);
        if (baseUrl && normalizedBaseUrl !== baseUrl.replace(/\/+$/, '')) {
          if (Number(dep.channel?.type) === 40) {
            errors.push(`${label} 的豆包 Base URL 请填写到域名，例如 https://ark.cn-beijing.volces.com，不要带 /api/v3`);
          } else if (Number(dep.channel?.type) === 20) {
            errors.push(`${label} 的 OpenRouter Base URL 请填写 https://openrouter.ai/api，不要带 /v1`);
          }
        }
        if (!key) {
          errors.push(`${label} 的密钥不能为空`);
        }
      }
      if (!channelName) {
        errors.push(`${label} 的渠道名称不能为空`);
      }
      if (!Number.isFinite(priority)) {
        errors.push(`${label} 的优先级必须是数字`);
      }
      if (!Number.isFinite(weight) || weight <= 0) {
        errors.push(`${label} 的权重必须大于 0`);
      }
      if (!Number.isFinite(dailyLimit) || dailyLimit < 0) {
        errors.push(`${label} 的每日 Token 限额不能小于 0`);
      }
      if (
        !Number.isFinite(softLimitRatio) ||
        softLimitRatio <= 0 ||
        softLimitRatio > 1
      ) {
        errors.push(`${label} 的软限额比例必须在 0 到 1 之间`);
      }
      if (
        !Number.isFinite(hardLimitRatio) ||
        hardLimitRatio <= 0 ||
        hardLimitRatio > 1
      ) {
        errors.push(`${label} 的硬限额比例必须在 0 到 1 之间`);
      }
      if (
        Number.isFinite(softLimitRatio) &&
        Number.isFinite(hardLimitRatio) &&
        softLimitRatio >= hardLimitRatio
      ) {
        errors.push(`${label} 的软限额比例必须小于硬限额比例`);
      }
    });

    const virtualModelNames = new Set();
    virtualModels.forEach((vm, index) => {
      const name = String(vm.name || '').trim();
      const label = name || `第 ${index + 1} 个虚拟模型`;
      const fallbackOrder = Array.isArray(vm.fallback_order)
        ? vm.fallback_order.map((id) => String(id || '').trim()).filter(Boolean)
        : [];
      const uniqueOrder = new Set(fallbackOrder);
      const routingMode = vm.routing_mode || ROUTING_MODE_WEIGHTED;
      const fixedDeployment = String(vm.fixed_deployment || '').trim();

      if (!name) {
        errors.push(`虚拟模型 #${index + 1} 的名称不能为空`);
      } else if (virtualModelNames.has(name)) {
        errors.push(`虚拟模型名称重复：${name}`);
      }
      if (name) {
        virtualModelNames.add(name);
      }
      if (vm.enabled !== false && fallbackOrder.length === 0) {
        errors.push(`${label} 至少需要绑定一个真实模型`);
      }
      if (uniqueOrder.size !== fallbackOrder.length) {
        errors.push(`${label} 绑定了重复的真实模型`);
      }
      fallbackOrder.forEach((deploymentId) => {
        if (!deploymentIds.has(deploymentId)) {
          errors.push(`${label} 引用了不存在的真实模型：${deploymentId}`);
        }
      });
      if (vm.enabled !== false && routingMode === ROUTING_MODE_FIXED) {
        if (!fixedDeployment) {
          errors.push(`${label} 固定模型模式需要选择一个固定目标`);
        } else if (!uniqueOrder.has(fixedDeployment)) {
          errors.push(`${label} 固定目标必须是已绑定的真实模型：${fixedDeployment}`);
        } else if (!enabledDeploymentIds.has(fixedDeployment)) {
          errors.push(`${label} 固定目标需要是已启用的真实模型：${fixedDeployment}`);
        }
      }
      if (
        vm.enabled !== false &&
        fallbackOrder.length > 0 &&
        !fallbackOrder.some((deploymentId) => enabledDeploymentIds.has(deploymentId))
      ) {
        errors.push(`${label} 至少需要一个已启用的真实模型`);
      }
    });

    return errors;
  };

  const saveConfig = async () => {
    const validationErrors = getConfigValidationErrors(config);
    if (validationErrors.length > 0) {
      showError(validationErrors.slice(0, 3).join('；'));
      return;
    }

    setSaving(true);
    try {
      const res = await API.post('/api/fallback/editor/config', config);
      const { success, message, data, backup_path } = res.data;
      if (success) {
        setConfig(normalizeConfig(data));
        await loadDeploymentStatuses(true);
        showSuccess(
          backup_path
            ? `虚拟模型配置已保存，旧配置已备份到 ${backup_path}`
            : '虚拟模型配置已保存'
        );
      } else {
        showError(message || '保存虚拟模型配置失败');
      }
    } catch (error) {
      showError(error.message || '保存虚拟模型配置失败');
    } finally {
      setSaving(false);
    }
  };

  const saveVirtualModel = async (vmIndex) => {
    const vm = config.virtual_models[vmIndex];
    if (!vm) return;

    setSavingVirtualModel(vm.name);
    try {
      const deploymentIds = new Set(vm.fallback_order);
      const payload = {
        enabled: true,
        virtual_models: [vm],
        deployments: config.deployments.filter(d => deploymentIds.has(d.id)),
        alert: config.alert,
        smart_sort: config.smart_sort,
      };

      const validationErrors = getConfigValidationErrors(payload);
      if (validationErrors.length > 0) {
        showError(validationErrors.slice(0, 3).join('；'));
        return;
      }

      const res = await API.post('/api/fallback/editor/config', payload);
      const { success, message, data } = res.data;
      if (success) {
        const updatedConfig = normalizeConfig(data);
        setConfig(prev => ({
          ...prev,
          virtual_models: prev.virtual_models.map((v, i) =>
            i === vmIndex ? updatedConfig.virtual_models[0] : v
          ),
          deployments: [
            ...prev.deployments.filter(d => !deploymentIds.has(d.id)),
            ...updatedConfig.deployments,
          ],
        }));
        await loadDeploymentStatuses(true);
        showSuccess(`虚拟模型 ${vm.name} 已保存`);
      } else {
        showError(message || '保存虚拟模型失败');
      }
    } catch (error) {
      showError(error.message || '保存虚拟模型失败');
    } finally {
      setSavingVirtualModel('');
    }
  };

  const computeDiff = () => {
    const lines = [];
    const vmCount = config.virtual_models.length;
    const depCount = config.deployments.length;
    lines.push(`虚拟模型: ${vmCount} 个`);
    lines.push(`真实模型: ${depCount} 个`);
    config.virtual_models.forEach((vm, i) => {
      lines.push(`  ${i + 1}. ${vm.name} — ${vm.enabled ? '启用' : '停用'} ${vm.description ? `(${vm.description})` : ''}`);
      const routingText =
        vm.routing_mode === ROUTING_MODE_FIXED
          ? '固定模型'
          : vm.routing_mode === ROUTING_MODE_SEQUENTIAL
            ? '按顺序'
            : '按权重';
      lines.push(`     路由: ${routingText}`);
      (vm.fallback_order || []).forEach((depId) => {
        const dep = config.deployments.find(d => d.id === depId);
        if (dep) {
          const fixedMark =
            vm.routing_mode === ROUTING_MODE_FIXED && vm.fixed_deployment === depId
              ? '，固定目标'
              : '';
          lines.push(`     - ${dep.real_model || depId} (渠道: ${dep.channel?.name || '-'}${fixedMark})`);
        }
      });
    });
    setDiffContent(lines);
    setDiffModalOpen(true);
  };

  const testAllDeployments = async () => {
    if (!config?.deployments?.length) {
      showError('没有可测试的真实模型');
      return;
    }

    setTesting(true);
    setTestResults({});
    showInfo('正在测试真实模型，请稍候...');
    try {
      for (const dep of config.deployments) {
        if (!dep.channel_id) {
          setTestResults((oldState) => ({
            ...oldState,
            [dep.id]: {
              success: false,
              message: '请先保存，生成渠道后再测试',
            },
          }));
          continue;
        }

        try {
          const model = encodeURIComponent(dep.real_model || '');
          const res = await API.get(`/api/channel/test/${dep.channel_id}?model=${model}`, {
            timeout: 30000,
          });
          const { success, message, time } = res?.data || {};
          setTestResults((oldState) => ({
            ...oldState,
            [dep.id]: {
              success: !!success,
              message: message || (success ? '测试通过' : '测试失败'),
              time,
            },
          }));
        } catch (error) {
          const isTimeout = error?.code === 'ECONNABORTED';
          setTestResults((oldState) => ({
            ...oldState,
            [dep.id]: {
              success: false,
              message: isTimeout
                ? '测试超时，已跳过该真实模型'
                : error?.response?.data?.message || error.message || '测试失败',
            },
          }));
        }
      }
      showSuccess('真实模型测试完成');
    } finally {
      setTesting(false);
    }
  };

  const testSingleDeployment = async (dep) => {
    if (!dep?.channel_id) {
      setTestResults((old) => ({
        ...old,
        [dep.id]: { success: false, message: '请先保存，生成渠道后再测试' },
      }));
      return;
    }

    try {
      const model = encodeURIComponent(dep.real_model || '');
      const res = await API.get(
        `/api/channel/test/${dep.channel_id}?model=${model}`,
        { timeout: 30000 }
      );
      const { success, message, time } = res?.data || {};
      setTestResults((old) => ({
        ...old,
        [dep.id]: {
          success: !!success,
          message: message || (success ? '测试通过' : '测试失败'),
          time,
        },
      }));
    } catch (error) {
      const isTimeout = error?.code === 'ECONNABORTED';
      setTestResults((old) => ({
        ...old,
        [dep.id]: {
          success: false,
          message: isTimeout
            ? '测试超时'
            : error?.response?.data?.message || error.message || '测试失败',
        },
      }));
    }
  };

  const batchAction = async (action) => {
    const deploymentIds = config.deployments
      .filter((dep) => dep.enabled !== false && dep.channel_id)
      .map((dep) => dep.id);
    if (deploymentIds.length === 0) {
      showError('没有可操作的真实模型');
      return;
    }

    setBatchLoading(action);
    try {
      const url = action === 'recover'
        ? '/api/fallback/deployments/batch-recover'
        : '/api/fallback/deployments/batch-cooldown';
      const payload = { deployment_ids: deploymentIds };
      if (action === 'cooldown') {
        payload.duration_seconds = 300;
      }

      const res = await API.post(url, payload);
      const { success, results, message } = res.data;
      if (success) {
        const succeed = results.filter((r) => r.success).length;
        showSuccess(`${action === 'recover' ? '恢复' : '冷却'}完成：${succeed}/${deploymentIds.length} 个成功`);
        await loadDeploymentStatuses(true);
      } else {
        showError(message || `批量${action === 'recover' ? '恢复' : '冷却'}失败`);
      }
    } catch (error) {
      showError(error.message || `批量${action === 'recover' ? '恢复' : '冷却'}失败`);
    } finally {
      setBatchLoading('');
    }
  };

  const runDeploymentAction = async (dep, action) => {
    if (!dep?.channel_id) {
      showError('请先保存，生成渠道后再操作部署状态');
      return;
    }

    const actionKey = `${dep.id}:${action}`;
    setActingDeployment(actionKey);
    try {
      let url = `/api/fallback/deployments/${encodeURIComponent(dep.id)}`;
      if (action === 'cooldown') {
        url += '/cooldown?duration_seconds=300';
      } else {
        url += '/recover';
      }

      const res = await API.post(url);
      const { success, message } = res.data || {};
      if (success === false) {
        throw new Error(message || '部署状态操作失败');
      }

      await loadDeploymentStatuses(true);
      showSuccess(
        action === 'cooldown' ? '已设置冷却' : '已恢复并重置当前周期额度'
      );
    } catch (error) {
      showError(error.message || '部署状态操作失败');
    } finally {
      setActingDeployment('');
    }
  };

  const formatStatusTime = (value) => {
    if (!value) {
      return '-';
    }
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) {
      return String(value);
    }
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

  if (loading && !config) {
    return (
      <div className='fallback-config-panel fallback-config-loading'>
        <Loader active inline='centered' />
      </div>
    );
  }

  if (!config) {
    return (
      <div className='fallback-config-panel'>
        <Message warning>未加载到虚拟模型配置</Message>
        <Button icon labelPosition='left' onClick={loadConfig} loading={loading}>
          <Icon name='refresh' />
          重新加载
        </Button>
      </div>
    );
  }

  return (
    <>
    <Message warning className='fallback-config-legacy-banner'>
      <Icon name='exclamation triangle' />
      <Message.Content>
        <Message.Header>已启用新版三层虚拟模型网关</Message.Header>
        <p>
          当前配置使用 <code>pools / strategy</code> 新结构，本编辑器仍基于旧版
          <code> fallback_order / fixed_deployment / routing_mode</code> 字段，可能无法正确保存新版配置。
        </p>
        <p>
          请通过编辑 <code>data/fallback.json</code> 并调用 <code>POST /api/fallback/config/reload</code> 修改配置，
          或前往「三层网关状态」页面查看运行状态。
        </p>
      </Message.Content>
    </Message>
    <div className='fallback-config-panel'>
      <div className='fallback-config-toolbar'>
        <div>
          <Header as='h3' className='fallback-config-title'>
            虚拟模型
          </Header>
           <div className='fallback-config-count'>
             {config.virtual_models.length} 个虚拟模型，{config.deployments.length} 个真实模型
           </div>
            <div style={{ fontSize: 12, color: '#868b94', marginTop: 2 }}>
              {config.virtual_models.length > 1 ? '' : '⚠️ 如只看到一个模型，请检查后端数据是否正常返回多个'}
            </div>
        </div>
        <div className='fallback-config-actions'>
          <Button icon labelPosition='left' onClick={addVirtualModel}>
            <Icon name='plus' />
            添加虚拟模型
          </Button>
          <Button icon labelPosition='left' onClick={loadConfig} loading={loading}>
            <Icon name='refresh' />
            刷新
          </Button>
          <Button
            icon
            labelPosition='left'
            onClick={testAllDeployments}
            loading={testing}
            disabled={testing || saving}
          >
            <Icon name='play' />
            测试所有
          </Button>
          <Button
            icon
            labelPosition='left'
            onClick={() => batchAction('recover')}
            loading={batchLoading === 'recover'}
            disabled={!!batchLoading}
          >
            <Icon name='undo' />
            全部恢复
          </Button>
          <Button
            icon
            labelPosition='left'
            onClick={() => batchAction('cooldown')}
            loading={batchLoading === 'cooldown'}
            disabled={!!batchLoading}
          >
            <Icon name='pause' />
            全部冷却
          </Button>
          <Popup
            size='mini'
            content='查看当前虚拟模型和真实模型配置摘要，不会保存改动'
            trigger={
              <Button
                icon
                labelPosition='left'
                onClick={computeDiff}
              >
                <Icon name='search' />
                预览变更
              </Button>
            }
          />
          <Button
            primary
            icon
            labelPosition='left'
            onClick={saveConfig}
            loading={saving}
            disabled={saving}
          >
            <Icon name='save' />
            保存
          </Button>
        </div>
      </div>

      <Divider />

      <div className='fallback-virtual-list'>
        {config.virtual_models.map((vm, vmIndex) => {
          const vmKey = getVirtualModelKey(vm, vmIndex);
          const vmExpanded = !!expandedVirtualModels[vmKey];
          const modelCount = (vm.fallback_order || []).length;
          const routingMode = vm.routing_mode || ROUTING_MODE_WEIGHTED;
          const isSequentialMode = routingMode === ROUTING_MODE_SEQUENTIAL;
          const isFixedMode = routingMode === ROUTING_MODE_FIXED;
          const orderEditorOpen =
            isSequentialMode && !!orderingVirtualModels[vmKey];

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
                    {vm.description ? ` - ${vm.description}` : ''}
                  </div>
                </div>
                <div className='fallback-virtual-summary-actions'>
                  <Label basic color={vm.enabled ? 'green' : 'grey'}>
                    {vm.enabled ? '启用' : '停用'}
                  </Label>
                  <Button
                    type='button'
                    basic
                    compact
                    icon='copy'
                    onClick={() => copyVirtualModel(vmIndex)}
                    title='复制此虚拟模型'
                  />
                </div>
              </div>

              {vmExpanded && (
                <div className='fallback-virtual-body'>
                  <div className='fallback-virtual-header'>
                    <Form className='fallback-virtual-form'>
                      <Form.Group widths='equal'>
                        <Form.Input
                          label='虚拟模型名'
                          value={vm.name}
                          onChange={(_, { value }) =>
                            setVirtualModel(vmIndex, 'name', value)
                          }
                        />
                        <Form.Input
                          label='描述'
                          value={vm.description}
                          onChange={(_, { value }) =>
                            setVirtualModel(vmIndex, 'description', value)
                          }
                        />
                      </Form.Group>
                    </Form>
                    <div className='fallback-virtual-controls'>
                      <Checkbox
                        toggle
                        label='启用'
                        checked={!!vm.enabled}
                        onChange={(_, { checked }) =>
                          setVirtualModel(vmIndex, 'enabled', checked)
                        }
                      />
                      <Button
                        basic
                        color='red'
                        icon='trash'
                        onClick={() => removeVirtualModel(vmIndex)}
                      />
                    </div>
                  </div>

                  <div className='fallback-routing-panel'>
                    <div className='fallback-routing-actions'>
                      <div
                        className='fallback-routing-mode-grid'
                        role='group'
                        aria-label='路由模式'
                      >
                        {ROUTING_MODE_OPTIONS.map((mode) => {
                          const optionMeta = ROUTING_MODE_META[mode];
                          const optionActive = routingMode === mode;
                          return (
                            <button
                              type='button'
                              key={mode}
                              className={`fallback-routing-option ${optionMeta.color} ${
                                optionActive ? 'active' : ''
                              }`}
                              onClick={() =>
                                setVirtualModelRoutingMode(vmIndex, vmKey, mode)
                              }
                            >
                              <span className='fallback-routing-option-head'>
                                <Icon name={optionMeta.icon} />
                                <strong>{optionMeta.title}</strong>
                                {optionActive && (
                                  <span className='fallback-routing-current'>
                                    当前
                                  </span>
                                )}
                              </span>
                              <span>{optionMeta.detail}</span>
                            </button>
                          );
                        })}
                      </div>
                      {isSequentialMode && (
                        <Button
                          type='button'
                          basic
                          icon
                          labelPosition='left'
                          className='fallback-routing-edit'
                          onClick={() => toggleOrderEditor(vmKey)}
                        >
                          <Icon name='exchange' />
                          {orderEditorOpen ? '完成排序' : '编辑顺序'}
                        </Button>
                      )}
                    </div>
                  </div>

                  <div className='fallback-deployment-list'>
                    {(vm.fallback_order || []).map((deploymentId, orderIndex) => {
                      const dep = deploymentsById[deploymentId];
                      if (!dep) {
                        return null;
                      }

                      const deploymentKey = getDeploymentKey(vmIndex, deploymentId);
                      const depExpanded = !!expandedDeployments[deploymentKey];
                      const keyVisible = !!visibleKeys[dep.id];
                      const testResult = testResults[dep.id];
                      const deploymentStatus = deploymentStatuses[dep.id];
                      const statusMeta = getDeploymentStatusMeta(deploymentStatus);
                      const cooldownActionKey = `${dep.id}:cooldown`;
                      const recoverActionKey = `${dep.id}:recover`;
                      const statusActionDisabled = saving || !dep.channel_id;
                      const isFixedDeployment =
                        isFixedMode && vm.fixed_deployment === dep.id;
                      const ownerNames = getDeploymentOwnerNames(
                        config.virtual_models,
                        dep.id
                      );
                      const ownerText = ownerNames.join(' / ');

                      return (
                        <div
                          className={`fallback-deployment-panel ${
                            isFixedDeployment ? 'fixed-active' : ''
                          } ${highlightDeployment === dep.id ? 'fallback-highlight' : ''}`}
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
                                {dep.quota_mode === 'free' ? '用完即换' : '限额 ' + dep.daily_limit_tokens.toLocaleString()}
                              </Label>
                              {ownerNames.length > 1 && (
                                <Label
                                  basic
                                  size='mini'
                                  color='orange'
                                  title={`共享部署：${ownerText}`}
                                >
                                  共享 {ownerNames.length}
                                </Label>
                              )}
                              {testResult && (
                                <Label
                                  basic
                                  size='mini'
                                  color={testResult.success ? 'green' : 'red'}
                                >
                                  {testResult.success
                                    ? `通过 ${testResult.time || 0}s`
                                    : '失败'}
                                </Label>
                              )}
                            </div>
                            <div className='fallback-deployment-controls'>
                              {isFixedMode && (
                                <Button
                                  type='button'
                                  basic={!isFixedDeployment}
                                  compact
                                  color='purple'
                                  icon={isFixedDeployment ? 'check circle' : 'bullseye'}
                                  title={
                                    isFixedDeployment
                                      ? '当前固定模型'
                                      : '设为固定模型'
                                  }
                                  onClick={() =>
                                    setVirtualModelFixedDeployment(vmIndex, dep.id)
                                  }
                                />
                              )}
                              {orderEditorOpen && (
                                <div className='fallback-order-actions'>
                                  <Button
                                    type='button'
                                    basic
                                    compact
                                    icon='arrow up'
                                    title='上移'
                                    disabled={orderIndex === 0}
                                    onClick={() =>
                                      moveDeploymentInVirtualModel(
                                        vmIndex,
                                        dep.id,
                                        -1
                                      )
                                    }
                                  />
                                  <Button
                                    type='button'
                                    basic
                                    compact
                                    icon='arrow down'
                                    title='下移'
                                    disabled={orderIndex === modelCount - 1}
                                    onClick={() =>
                                      moveDeploymentInVirtualModel(
                                        vmIndex,
                                        dep.id,
                                        1
                                      )
                                    }
                                  />
                                </div>
                              )}
                              <div className='fallback-state-actions'>
                                <Button
                                  type='button'
                                  basic
                                  compact
                                  icon='pause'
                                  title='冷却 5 分钟，不重置额度'
                                  loading={actingDeployment === cooldownActionKey}
                                  disabled={statusActionDisabled || !!actingDeployment}
                                  onClick={() => runDeploymentAction(dep, 'cooldown')}
                                />
                                <Button
                                  type='button'
                                  basic
                                  compact
                                  color='green'
                                  icon='undo'
                                  title='恢复部署并重置当前周期额度'
                                  loading={actingDeployment === recoverActionKey}
                                  disabled={statusActionDisabled || !!actingDeployment}
                                  onClick={() => runDeploymentAction(dep, 'recover')}
                                />
                                <Button
                                  type='button'
                                  basic
                                  compact
                                  color='blue'
                                  icon='save'
                                  title='保存此真实模型的配置'
                                  loading={savingVirtualModel === dep.id}
                                  disabled={savingVirtualModel === dep.id || saving}
                                  onClick={() => saveVirtualModel(vmIndex)}
                                />
                                <Button
                                  type='button'
                                  basic
                                  compact
                                  color='blue'
                                  icon='play'
                                  title='测试此真实模型'
                                  loading={testingSingle === dep.id}
                                  disabled={testing || testingSingle !== null}
                                  onClick={async () => {
                                    if (!dep?.channel_id) {
                                      setTestResults((old) => ({
                                        ...old,
                                        [dep.id]: { success: false, message: '请先保存，生成渠道后再测试' },
                                      }));
                                      showError('请先保存，生成渠道后再测试');
                                      return;
                                    }
                                    setTestingSingle(dep.id);
                                    const startTime = Date.now();
                                    try {
                                      const model = encodeURIComponent(dep.real_model || '');
                                      const res = await API.get(
                                        `/api/channel/test/${dep.channel_id}?model=${model}`,
                                        { timeout: 30000 }
                                      );
                                      const { success, message, time } = res?.data || {};
                                      const elapsed = ((time ?? Date.now() - startTime) / 1000).toFixed(2);
                                      setTestResults((old) => ({
                                        ...old,
                                        [dep.id]: {
                                          success: !!success,
                                          message: message || (success ? '测试通过' : '测试失败'),
                                          time,
                                        },
                                      }));
                                      if (success) {
                                        showSuccess(`测试成功，耗时 ${elapsed}s`);
                                      } else {
                                        showError(`测试失败：${message || '未知原因'}`);
                                      }
                                    } catch (error) {
                                      const isTimeout = error?.code === 'ECONNABORTED';
                                      const failMessage = isTimeout
                                        ? '测试超时'
                                        : error?.response?.data?.message || error.message || '测试失败';
                                      setTestResults((old) => ({
                                        ...old,
                                        [dep.id]: { success: false, message: failMessage },
                                      }));
                                      showError(`测试失败：${failMessage}`);
                                    } finally {
                                      setTestingSingle(null);
                                    }
                                  }}
                                />
                              </div>
                              <Checkbox
                                toggle
                                label='启用'
                                checked={!!dep.enabled}
                                onChange={(_, { checked }) =>
                                  setDeployment(dep.id, 'enabled', checked)
                                }
                              />
                              <Button
                                basic
                                color='red'
                                icon='trash'
                                onClick={() =>
                                  removeDeploymentFromVirtualModel(vmIndex, dep.id)
                                }
                              />
                            </div>
                          </div>

                          <div className='fallback-state-note'>
                            {statusMeta.detail}
                          </div>

                          {testResult && (
                            <div
                              className={`fallback-test-result ${
                                testResult.success ? 'success' : 'failed'
                              }`}
                            >
                              {testResult.message}
                            </div>
                          )}

                          {depExpanded && (
                            <Form className='fallback-deployment-form'>
                              <div className='fallback-config-guard'>
                                <Icon name='info circle' />
                                <span>
                                  当前归属：{ownerText || vm.name || '当前虚拟模型'}。受控额度会按本地 Token 限额提前切换；用完即换不设置本地额度，依赖上游报错后立即 fallback。
                                </span>
                              </div>
                              <Form.Group widths='equal'>
                                <Form.Dropdown
                                  label='从已有渠道选择'
                                  selection
                                  clearable
                                  search
                                  options={channelTemplateOptions}
                                  placeholder='展开已有渠道和模型'
                                  value={
                                    dep.channel_id
                                      ? `${dep.channel_id}::${dep.real_model || ''}`
                                      : ''
                                  }
                                  onChange={(_, { value }) =>
                                    applyDeploymentTemplate(dep.id, value)
                                  }
                                />
                                <Form.Field />
                                <Form.Field />
                              </Form.Group>
                              <Form.Group widths='equal'>
                                <Form.Input
                                  label='真实模型名'
                                  value={dep.real_model || ''}
                                  onChange={(_, { value }) =>
                                    setDeployment(dep.id, 'real_model', value)
                                  }
                                />
                                <Form.Input
                                  label='接口地址'
                                  value={dep.channel?.base_url || ''}
                                  onChange={(_, { value }) =>
                                    setDeploymentChannel(dep.id, 'base_url', value)
                                  }
                                />
                                <Form.Field>
                                  <label>密钥</label>
                                  <Input
                                    type={keyVisible ? 'text' : 'password'}
                                    value={dep.channel?.key_masked || ''}
                                    onChange={(_, { value }) =>
                                      setDeploymentChannel(dep.id, 'key_masked', value)
                                    }
                                    action={
                                      <Button
                                        type='button'
                                        icon={keyVisible ? 'eye slash' : 'eye'}
                                        onClick={() => toggleKeyVisible(dep.id)}
                                      />
                                    }
                                  />
                                </Form.Field>
                              </Form.Group>
                              <Form.Group widths='equal'>
                                <Form.Input
                                  label='渠道名称'
                                  value={dep.channel?.name || ''}
                                  onChange={(_, { value }) =>
                                    setDeploymentChannel(dep.id, 'name', value)
                                  }
                                />
                                <Form.Input
                                  label='优先级'
                                  type='number'
                                  value={dep.priority}
                                  onChange={(_, { value }) =>
                                    setDeployment(dep.id, 'priority', Number(value))
                                  }
                                />
                                <Form.Input
                                  label={
                                    isSequentialMode
                                      ? '权重（权重模式生效）'
                                      : '权重'
                                  }
                                  type='number'
                                  value={dep.weight}
                                  onChange={(_, { value }) =>
                                    setDeployment(dep.id, 'weight', Number(value))
                                  }
                                />
                                <Form.Dropdown
                                  label='额度模式'
                                  selection
                                  value={dep.quota_mode || 'controlled'}
                                  options={[
                                    { key: 'controlled', value: 'controlled', text: '受控（自定限额）' },
                                    { key: 'free', value: 'free', text: '用完就换（不设限额）' },
                                  ]}
                                  onChange={(_, { value }) =>
                                    setDeploymentQuotaMode(dep.id, value)
                                  }
                                />
                                <Form.Input
                                  label='每日 Token 限额'
                                  type='number'
                                  disabled={dep.quota_mode === 'free'}
                                  value={dep.quota_mode === 'free' ? 0 : dep.daily_limit_tokens}
                                  onChange={(_, { value }) =>
                                    setDeployment(
                                      dep.id,
                                      'daily_limit_tokens',
                                      Number(value)
                                    )
                                  }
                                />
                              </Form.Group>
                            </Form>
                          )}
                        </div>
                      );
                    })}
                  </div>

                  <Button
                    className='fallback-add-real-model'
                    basic
                    icon
                    labelPosition='left'
                    onClick={() => addDeploymentToVirtualModel(vmIndex)}
                  >
                    <Icon name='plus' />
                    添加真实模型
                  </Button>
                </div>
              )}
            </section>
          );
        })}
      </div>
    </div>

      <Modal
        open={diffModalOpen}
        onClose={() => setDiffModalOpen(false)}
        size='small'
      >
        <Modal.Header>配置变更预览</Modal.Header>
        <Modal.Content>
          <pre style={{ fontSize: '13px', lineHeight: 1.6, whiteSpace: 'pre-wrap', background: '#f8fafc', padding: '16px', borderRadius: '8px' }}>
            {diffContent.join('\n')}
          </pre>
        </Modal.Content>
        <Modal.Actions>
          <Button onClick={() => setDiffModalOpen(false)}>
            关闭
          </Button>
        </Modal.Actions>
      </Modal>
    </>
  );
};

export default FallbackConfigPanel;
