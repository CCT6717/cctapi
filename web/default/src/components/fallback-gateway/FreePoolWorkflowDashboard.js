import React from 'react';

const statusText = (complete) => (complete ? 'Ready' : 'Needs action');

const FreePoolWorkflowDashboard = ({ summary }) => {
  if (!summary) return null;

  const readinessLabel = `${summary.readinessScore}/${summary.readinessTotal} complete`;
  const riskCountLabel = summary.risks.length === 0
    ? 'No blocking risks'
    : `${summary.risks.length} risk${summary.risks.length > 1 ? 's' : ''}`;

  return (
    <section className='free-pool-workflow-dashboard'>
      <div className='free-pool-workflow-hero'>
        <div>
          <span className='free-pool-workflow-eyebrow'>FreeLLMAPI workflow</span>
          <h3>Integration readiness</h3>
          <p>
            Track whether cct/free can route traffic, whether providers are usable,
            and whether quota telemetry is safe to trust.
          </p>
        </div>
        <div className='free-pool-readiness-meter'>
          <strong>{readinessLabel}</strong>
          <span>{summary.readinessPercent}% ready</span>
          <div className='free-pool-readiness-track'>
            <i style={{ width: `${summary.readinessPercent}%` }} />
          </div>
        </div>
      </div>

      <div className='free-pool-workflow-metrics'>
        <div>
          <span>Ready providers</span>
          <strong>{summary.readyProviderCount} / {summary.providerCount}</strong>
        </div>
        <div>
          <span>Enabled deployments</span>
          <strong>{summary.enabledDeploymentCount} / {summary.deploymentCount}</strong>
        </div>
        <div>
          <span>Usage rows</span>
          <strong>{summary.usageRowCount}</strong>
        </div>
        <div>
          <span>Runtime risks</span>
          <strong>{riskCountLabel}</strong>
        </div>
      </div>

      <div className='free-pool-workflow-body'>
        <div className='free-pool-workflow-steps'>
          <h4>Setup checklist</h4>
          {summary.steps.map((step) => (
            <div
              className={`free-pool-workflow-step ${step.complete ? 'complete' : 'blocked'}`}
              key={step.key}
            >
              <span>{step.complete ? '✓' : '!'}</span>
              <div>
                <strong>{step.label}</strong>
                <em>{statusText(step.complete)} · {step.detail}</em>
              </div>
            </div>
          ))}
        </div>

        <div className='free-pool-workflow-actions-panel'>
          <h4>Recommended next actions</h4>
          <ol>
            {summary.nextActions.map((action) => (
              <li key={action}>{action}</li>
            ))}
          </ol>
        </div>
      </div>
    </section>
  );
};

export default FreePoolWorkflowDashboard;
