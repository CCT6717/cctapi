import React from 'react';
import FreePoolWorkflowDashboard from './FreePoolWorkflowDashboard';
import '../../pages/Fallback/Fallback.css';

const meta = {
  title: 'Fallback/FreePoolWorkflowDashboard',
  component: FreePoolWorkflowDashboard,
};

export default meta;

const readySummary = {
  readinessScore: 4,
  readinessTotal: 4,
  readinessPercent: 100,
  providerCount: 5,
  readyProviderCount: 3,
  deploymentCount: 8,
  enabledDeploymentCount: 8,
  usageRowCount: 24,
  invalidRuntimeCount: 0,
  risks: [],
  nextActions: ['通过 cct/free 发送一次小请求，并观察用量是否增长。'],
  steps: [
    { key: 'virtual-model', label: 'cct/free 虚拟模型', complete: true, detail: '已启用' },
    { key: 'providers', label: '就绪供应商', complete: true, detail: '3 / 5' },
    { key: 'deployments', label: '已生成部署', complete: true, detail: '8 / 8' },
    { key: 'usage', label: '用量统计', complete: true, detail: '24 条' },
  ],
};

const needsActionSummary = {
  readinessScore: 1,
  readinessTotal: 4,
  readinessPercent: 25,
  providerCount: 4,
  readyProviderCount: 0,
  deploymentCount: 0,
  enabledDeploymentCount: 0,
  usageRowCount: 0,
  invalidRuntimeCount: 0,
  risks: [
    { key: 'no-ready-provider', level: 'critical', text: '没有已启用且可用的供应商。' },
    { key: 'usage-unavailable', level: 'warning', text: '用量统计不可用。' },
  ],
  nextActions: [
    '至少启用一个已保存密钥或支持免 key 的供应商。',
    '同步免费池以生成部署。',
    '检查免费池用量接口，再依赖额度信号。',
  ],
  steps: [
    { key: 'virtual-model', label: 'cct/free 虚拟模型', complete: true, detail: '已启用' },
    { key: 'providers', label: '就绪供应商', complete: false, detail: '0 / 4' },
    { key: 'deployments', label: '已生成部署', complete: false, detail: '0 / 0' },
    { key: 'usage', label: '用量统计', complete: false, detail: '不可用' },
  ],
};

export const Ready = {
  render: () => (
    <div className='fallback-page' style={{ margin: 0 }}>
      <FreePoolWorkflowDashboard summary={readySummary} />
    </div>
  ),
};

export const NeedsAction = {
  render: () => (
    <div className='fallback-page' style={{ margin: 0 }}>
      <FreePoolWorkflowDashboard summary={needsActionSummary} />
    </div>
  ),
};
