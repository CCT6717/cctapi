// ============================================================
// SummaryBar.js — Fallback 页面摘要栏
// ============================================================

import React from 'react';

const SummaryBar = ({ summary }) => {
  if (!summary) return null;
  const parts = [];
  if (summary.switch_count > 0) {
    parts.push(`过去 1 小时内：${summary.switch_count} 次回退切换`);
  }
  const rateLimitedItems = (summary.rate_limited || [])
    .filter((item) => item.count > 0)
    .map((item) => `${item.deployment_id} 被限流 ${item.count} 次`);
  parts.push(...rateLimitedItems);
  const coolingDownItems = (summary.cooling_down || [])
    .map((depId) => `${depId} 冷却中`);
  parts.push(...coolingDownItems);

  if (parts.length === 0) return null;

  const hasIssue = summary.switch_count > 0 || coolingDownItems.length > 0;

  return (
    <div className={`fallback-summary-bar ${hasIssue ? 'warning' : 'info'}`}>
      <span className='fallback-summary-icon'>{hasIssue ? '⚠️' : '✅'}</span>
      <span className='fallback-summary-text'>{parts.join('，')}</span>
    </div>
  );
};

export default SummaryBar;
