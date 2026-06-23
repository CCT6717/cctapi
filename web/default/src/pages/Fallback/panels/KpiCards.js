import React from 'react';

const KpiCards = ({ configMeta, runtimeHealth, summary }) => {
  const healthLevel = runtimeHealth?.level || 'normal';
  const hasAlerts =
    (runtimeHealth?.coolingRows?.length || 0) > 0 ||
    (runtimeHealth?.quotaExhaustedRows?.length || 0) > 0 ||
    (summary?.switch_count || 0) > 0;

  return (
    <div className='fallback-runtime-grid'>
      <article className={`fallback-runtime-card ${healthLevel}`}>
        <span>系统健康</span>
        <strong>{runtimeHealth?.title || '运行平稳'}</strong>
        <small>{runtimeHealth?.message || '加载中...'}</small>
      </article>
      <article className='fallback-runtime-card normal'>
        <span>虚拟模型</span>
        <strong>{configMeta?.virtualOrder?.length || 0}</strong>
        <small>个已配置</small>
      </article>
      <article className='fallback-runtime-card normal'>
        <span>真实部署</span>
        <strong>{configMeta?.deploymentMap ? Object.keys(configMeta.deploymentMap).length : 0}</strong>
        <small>个 deployment</small>
      </article>
      <article className={`fallback-runtime-card ${hasAlerts ? 'warning' : 'normal'}`}>
        <span>近 1 小时异常</span>
        <strong>{summary?.switch_count || 0}</strong>
        <small>次切换事件</small>
      </article>
    </div>
  );
};

export default KpiCards;
