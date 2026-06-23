// ============================================================
// Fallback/index.js — Fallback 页面主容器
// ============================================================

import React from 'react';
import { Link } from 'react-router-dom';
import { Button, Icon, Loader, Message, Popup } from 'semantic-ui-react';
import ModelEditor from '../../components/FallbackConfigPanel';
import FreeModelPool from '../../components/fallback-gateway/FreeModelPool';
import {
  GUIDE_SECTIONS,
  PANEL_ITEMS,
  PANEL_REFRESH_INTERVALS,
  formatInterval,
  formatTime,
} from './utils/fallbackHelpers';
import { useFallbackPage } from './hooks/useFallbackPage';
import SummaryBar from './panels/SummaryBar';
import StatusPanel from './panels/StatusPanel';
import MetricsPanel from './panels/MetricsPanel';
import ScoresPanel from './panels/ScoresPanel';
import AlertsPanel from './panels/AlertsPanel';
import LogsPanel from './panels/LogsPanel';
import KpiCards from './panels/KpiCards';
import './Fallback.css';

const Fallback = () => {
  const {
    // Router
    activePanel,

    // State
    loading,
    lastUpdated,
    alertEvents,
    metricsText,
    switchEvents,
    statusSort,
    actingDeployment,
    guideOpen,
    summary,
    metricSamples,
    metricRows,
    configMeta,

    // Setters
    setStatusSort,
    setGuideOpen,

    // Computed
    statusDisplayRows,
    runtimeMetrics,
    runtimeHealth,
    metricTrendData,
    scoreTrend,
    scoreTrendGroups,

    // Actions
    loadPanel,
    markAllAlertsRead,
    runDeploymentAction,
    exportMetricsCSV,

    // Meta
    admin,
    refreshInterval,
  } = useFallbackPage();

  const activePanelItem =
    PANEL_ITEMS.find((item) => item.key === activePanel) || PANEL_ITEMS[0];
  const refreshHint = `自动每 ${formatInterval(
    refreshInterval
  )} 刷新，点击可立即显示最新数据`;

  if (!admin) {
    return (
      <div className='fallback-page'>
        <Message warning>需要管理员权限才能查看 fallback 面板。</Message>
      </div>
    );
  }

  const renderActivePanel = () => {
    if (loading && activePanel !== 'gateway' && activePanel !== 'free-pool') {
      return (
        <div className='fallback-loading'>
          <Loader active inline='centered' />
        </div>
      );
    }

    switch (activePanel) {
      case 'free-pool':
        return <FreeModelPool />;
      case 'gateway':
        return <ModelEditor />;
      case 'metrics':
        return (
          <MetricsPanel
            runtimeMetrics={runtimeMetrics}
            metricTrendData={metricTrendData}
            runtimeHealth={runtimeHealth}
            metricSamples={metricSamples}
            metricsText={metricsText}
            metricRows={metricRows}
            summary={summary}
            exportMetricsCSV={exportMetricsCSV}
          />
        );
      case 'scores':
        return (
          <ScoresPanel
            scoreTrend={scoreTrend}
            scoreTrendGroups={scoreTrendGroups}
            loading={loading}
            loadPanel={loadPanel}
          />
        );
      case 'alerts':
        return (
          <AlertsPanel
            alertEvents={alertEvents}
            markAllAlertsRead={markAllAlertsRead}
          />
        );
      case 'logs':
        return <LogsPanel switchEvents={switchEvents} />;
      default:
        return (
          <StatusPanel
            statusDisplayRows={statusDisplayRows}
            statusSort={statusSort}
            setStatusSort={setStatusSort}
            actingDeployment={actingDeployment}
            runDeploymentAction={runDeploymentAction}
          />
        );
    }
  };

  return (
    <div className='fallback-page'>
      <section className='fallback-guide-panel' id='fallback-guide'>
        <div className='fallback-guide-head'>
          <div>
            <h2>CCT API Fallback 快速说明</h2>
            <p>
              给第一次接触这个项目的人看的配置说明：这里列出新增能力、配置位置和日常查看入口。
            </p>
          </div>
          <Button
            type='button'
            basic
            size='small'
            aria-expanded={guideOpen}
            aria-controls='fallback-guide-content'
            onClick={() => setGuideOpen((open) => !open)}
          >
            {guideOpen ? '收起说明' : '首次配置看这里'}
            <Icon name={guideOpen ? 'angle up' : 'arrow right'} />
          </Button>
        </div>
        {guideOpen && (
          <div className='fallback-guide-grid' id='fallback-guide-content'>
            {GUIDE_SECTIONS.map((section) => (
              <article className='fallback-guide-card' key={section.title}>
                <span className='fallback-guide-icon'>
                  <Icon name={section.icon} />
                </span>
                <div>
                  <h3>{section.title}</h3>
                  <ul>
                    {section.items.map((item) => (
                      <li key={item}>{item}</li>
                    ))}
                  </ul>
                </div>
              </article>
            ))}
          </div>
        )}
      </section>

      <KpiCards
        configMeta={configMeta}
        runtimeHealth={runtimeHealth}
        summary={summary}
      />

      <SummaryBar summary={summary} />

      <div className='fallback-page-header'>
        <div>
          <h1>Fallback 面板</h1>
          <p>{activePanelItem.description}</p>
          <div className='fallback-page-kicker'>
            <span>{activePanelItem.title}</span>
          </div>
        </div>
        <div className='fallback-header-actions'>
          <span>
            最后刷新：
            {lastUpdated ? formatTime(lastUpdated) : '-'}
          </span>
          <span>自动刷新：{formatInterval(refreshInterval)}</span>
          <Popup
            content='功能说明'
            position='bottom center'
            trigger={
              <Button
                as='a'
                href='#fallback-guide'
                basic
                icon
                size='small'
                className='fallback-help-trigger'
                aria-label='功能说明'
                onClick={() => setGuideOpen(true)}
              >
                <Icon name='hand pointer outline' />
              </Button>
            }
          />
          <Button
            basic
            icon
            size='small'
            title={refreshHint}
            onClick={() => loadPanel(true)}
          >
            <Icon name='refresh' />
          </Button>
        </div>
      </div>

      <nav className='fallback-panel-grid'>
        {PANEL_ITEMS.map((item) => (
          <Link
            key={item.key}
            to={`/fallback/${item.key}`}
            className={`fallback-nav-card ${
              activePanel === item.key ? 'active' : ''
            }`}
            style={{ '--panel-accent': item.accent }}
            title={item.description}
          >
            <span className='fallback-nav-icon'>
              <Icon name={item.icon} />
            </span>
            <span className='fallback-nav-content'>
              <span className='fallback-nav-top'>
                <strong>{item.title}</strong>
                <span className='fallback-nav-refresh-badge'>每 {formatInterval(PANEL_REFRESH_INTERVALS[item.key])}</span>
              </span>
            </span>
          </Link>
        ))}
      </nav>

      <section className='fallback-content-panel'>{renderActivePanel()}</section>
    </div>
  );
};

export default Fallback;
