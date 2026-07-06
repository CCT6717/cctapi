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
  nextActions: ['Run a small request through cct/free and watch usage grow.'],
  steps: [
    { key: 'virtual-model', label: 'cct/free virtual model', complete: true, detail: 'enabled' },
    { key: 'providers', label: 'ready providers', complete: true, detail: '3 / 5' },
    { key: 'deployments', label: 'generated deployments', complete: true, detail: '8 / 8' },
    { key: 'usage', label: 'usage telemetry', complete: true, detail: '24 rows' },
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
    { key: 'no-ready-provider', level: 'critical', text: 'No enabled provider has a stored key.' },
    { key: 'usage-unavailable', level: 'warning', text: 'Usage telemetry is unavailable.' },
  ],
  nextActions: [
    'Enable at least one provider with a stored key or keyless access.',
    'Sync the free pool to generate deployments.',
    'Check the free-pool usage endpoint before relying on quota signals.',
  ],
  steps: [
    { key: 'virtual-model', label: 'cct/free virtual model', complete: true, detail: 'enabled' },
    { key: 'providers', label: 'ready providers', complete: false, detail: '0 / 4' },
    { key: 'deployments', label: 'generated deployments', complete: false, detail: '0 / 0' },
    { key: 'usage', label: 'usage telemetry', complete: false, detail: 'unavailable' },
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
