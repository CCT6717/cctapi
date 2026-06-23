// ============================================================
// ScoresPanel.js — Fallback 排序分数面板
// ============================================================

import React from 'react';
import { Button, Card, Icon, Message } from 'semantic-ui-react';
import {
  formatNumber,
  getScoreBand,
  getScoreDeltaMeta,
  getScoreSeriesPoints,
} from '../utils/fallbackHelpers';
import { clampScore } from '../scoreUtils';

const ScoresPanel = ({
  scoreTrend,
  scoreTrendGroups,
  loading,
  loadPanel,
}) => (
  <>
    <div className='fallback-content-toolbar'>
      <div>
        <h2>排序分数</h2>
        <span>每个虚拟模型内按当前智能排序分数从高到低展示。</span>
      </div>
      <Button
        basic
        icon
        labelPosition='left'
        size='small'
        loading={loading}
        disabled={loading}
        onClick={() => loadPanel(true)}
      >
        <Icon name='refresh' />
        刷新
      </Button>
    </div>
    <Card fluid className='fallback-score-trend-card'>
      <Card.Content>
        <div className='fallback-score-card-head'>
          <div className='fallback-score-card-title'>
            <Card.Header>分数排序</Card.Header>
            {scoreTrend.rows.length > 0 && (
              <span>
                按综合得分从高到低排序
              </span>
            )}
          </div>
          {scoreTrend.scoreSummary && (
            <div className='fallback-score-stats'>
              <span>最高 {scoreTrend.scoreSummary.max.toFixed(1)}</span>
              <span>平均 {scoreTrend.scoreSummary.avg.toFixed(1)}</span>
              <span>最低 {scoreTrend.scoreSummary.min.toFixed(1)}</span>
            </div>
          )}
        </div>
        {scoreTrendGroups.length === 0 ? (
          <Message>暂无排序分数历史</Message>
        ) : (
          <div className='fallback-score-trend-rank'>
            {scoreTrendGroups.map((group) => (
              <section
                className='fallback-score-trend-group'
                key={group.virtualModel}
              >
                <div className='fallback-score-trend-group-head'>
                  <strong>{group.virtualModel}</strong>
                  <span>{formatNumber(group.items.length)} 个部署</span>
                </div>
                {group.items.map((item, index) => {
                  const deltaMeta = getScoreDeltaMeta(
                    scoreTrend.rows,
                    item.deploymentId
                  );
                  const series = getScoreSeriesPoints(
                    scoreTrend.rows,
                    item.deploymentId,
                    item.value
                  );
                  const rankClass =
                    index === 0
                      ? 'gold'
                      : index === 1
                      ? 'silver'
                      : index === 2
                      ? 'bronze'
                      : 'normal';

                  return (
                    <article
                      className={`fallback-score-trend-row ${
                        index === 0 ? 'is-top' : ''
                      }`}
                      data-score-band={getScoreBand(item.value)}
                      key={`${group.virtualModel}:${item.deploymentId}`}
                    >
                      <span className={`fallback-rank-badge ${rankClass}`}>
                        {String(index + 1).padStart(2, '0')}
                      </span>
                      <div className='fallback-score-trend-main'>
                        <div className='fallback-score-trend-name'>
                          <strong title={item.deploymentId}>
                            {item.deploymentId}
                          </strong>
                          {index === 0 && (
                            <span className='fallback-score-rank-top'>Top</span>
                          )}
                        </div>
                        <div className='fallback-score-trend-track'>
                          <span
                            style={{
                              width: `${clampScore(item.value)}%`,
                            }}
                          />
                        </div>
                      </div>
                      <div className='fallback-score-trend-meta'>
                        <strong>{item.value.toFixed(1)}</strong>
                        <span className={deltaMeta?.direction || 'flat'}>
                          <Icon name={deltaMeta?.icon || 'minus'} />
                          {deltaMeta?.text || '暂无'}
                        </span>
                        <em>{series.length} 点</em>
                      </div>
                    </article>
                  );
                })}
              </section>
            ))}
          </div>
        )}
      </Card.Content>
    </Card>
  </>
);

export default ScoresPanel;
