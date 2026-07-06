import React from 'react';

const statusText = (complete) => (complete ? '就绪' : '需要处理');

const FreePoolWorkflowDashboard = ({ summary }) => {
  if (!summary) return null;

  const readinessLabel = `${summary.readinessScore}/${summary.readinessTotal} 已完成`;
  const riskCountLabel = summary.risks.length === 0
    ? '无阻塞风险'
    : `${summary.risks.length} 项风险`;

  return (
    <section className='free-pool-workflow-dashboard'>
      <div className='free-pool-workflow-hero'>
        <div>
          <span className='free-pool-workflow-eyebrow'>FreeLLMAPI 工作流</span>
          <h3>接入就绪度</h3>
          <p>
            检查 cct/free 是否可以路由流量、供应商是否可用，以及额度统计是否可信。
          </p>
        </div>
        <div className='free-pool-readiness-meter'>
          <strong>{readinessLabel}</strong>
          <span>{summary.readinessPercent}% 就绪</span>
          <div className='free-pool-readiness-track'>
            <i style={{ width: `${summary.readinessPercent}%` }} />
          </div>
        </div>
      </div>

      <div className='free-pool-workflow-metrics'>
        <div>
          <span>就绪供应商</span>
          <strong>{summary.readyProviderCount} / {summary.providerCount}</strong>
        </div>
        <div>
          <span>已启用部署</span>
          <strong>{summary.enabledDeploymentCount} / {summary.deploymentCount}</strong>
        </div>
        <div>
          <span>用量记录</span>
          <strong>{summary.usageRowCount}</strong>
        </div>
        <div>
          <span>运行风险</span>
          <strong>{riskCountLabel}</strong>
        </div>
      </div>

      <div className='free-pool-workflow-body'>
        <div className='free-pool-workflow-steps'>
          <h4>配置检查清单</h4>
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
          <h4>建议下一步</h4>
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
