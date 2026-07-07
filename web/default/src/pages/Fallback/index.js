import React from 'react';
import { Link } from 'react-router-dom';
import { Alert, Spin } from 'antd';
import {
  Activity,
  ArrowRight,
  ArrowRightLeft,
  BarChart3,
  Bell,
  ChevronUp,
  Cloud,
  Edit,
  HelpCircle,
  Map,
  MousePointer,
  RefreshCw,
  Rocket,
  Server,
  Settings,
} from 'lucide-react';
import ModelEditor from '../../components/FallbackConfigPanel';
import FreeModelPool from '../../components/fallback-gateway/FreeModelPool';
import {
  ActionToolbar,
  AdminCard,
  IconButton,
  PageHeader,
  StatusTag,
  UiIcon,
} from '../../ui';
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

const PANEL_ICON_COMPONENTS = {
  gateway: Edit,
  'free-pool': Cloud,
  status: Server,
  metrics: Activity,
  scores: BarChart3,
  alerts: Bell,
  logs: ArrowRightLeft,
};

const GUIDE_ICON_COMPONENTS = {
  rocket: Rocket,
  settings: Settings,
  map: Map,
  'map signs': Map,
};

const renderPanelIcon = (item) => {
  const IconComponent = PANEL_ICON_COMPONENTS[item.key] || Server;
  return <UiIcon icon={IconComponent} />;
};

const renderGuideIcon = (section) => {
  const IconComponent = GUIDE_ICON_COMPONENTS[section.icon] || HelpCircle;
  return <UiIcon icon={IconComponent} />;
};

const Fallback = () => {
  const {
    activePanel,
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
    setStatusSort,
    setGuideOpen,
    statusDisplayRows,
    runtimeMetrics,
    runtimeHealth,
    metricTrendData,
    scoreTrend,
    scoreTrendGroups,
    loadPanel,
    markAllAlertsRead,
    runDeploymentAction,
    exportMetricsCSV,
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
        <Alert
          type='warning'
          showIcon
          title='需要管理员权限才能查看 fallback 面板。'
        />
      </div>
    );
  }

  const renderActivePanel = () => {
    if (loading && activePanel !== 'gateway' && activePanel !== 'free-pool') {
      return (
        <div className='fallback-loading'>
          <Spin />
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
      <AdminCard className='fallback-guide-panel' id='fallback-guide'>
        <div className='fallback-guide-head'>
          <div>
            <h2>CCT API Fallback 快速说明</h2>
            <p>
              给第一次接触这个项目的人看的配置说明：这里列出新增能力、配置位置和日常查看入口。
            </p>
          </div>
          <IconButton
            type='default'
            size='small'
            aria-expanded={guideOpen}
            aria-controls='fallback-guide-content'
            icon={guideOpen ? ChevronUp : ArrowRight}
            iconProps={{ size: 15 }}
            label={guideOpen ? '收起说明' : '首次配置看这里'}
            onClick={() => setGuideOpen((open) => !open)}
          >
            {guideOpen ? '收起说明' : '首次配置看这里'}
          </IconButton>
        </div>
        {guideOpen && (
          <div className='fallback-guide-grid' id='fallback-guide-content'>
            {GUIDE_SECTIONS.map((section) => (
              <article className='fallback-guide-card' key={section.title}>
                <span className='fallback-guide-icon'>
                  {renderGuideIcon(section)}
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
      </AdminCard>

      <KpiCards
        configMeta={configMeta}
        runtimeHealth={runtimeHealth}
        summary={summary}
      />

      <SummaryBar summary={summary} />

      <PageHeader
        className='fallback-page-header'
        title='Fallback 面板'
        description={activePanelItem.description}
        kicker={activePanelItem.title}
        actions={
          <ActionToolbar className='fallback-header-actions'>
            <span>
              最后刷新：
              {lastUpdated ? formatTime(lastUpdated) : '-'}
            </span>
            <StatusTag status='info' showDot={false}>
              自动刷新：{formatInterval(refreshInterval)}
            </StatusTag>
            <IconButton
              href='#fallback-guide'
              size='small'
              className='fallback-help-trigger'
              icon={MousePointer}
              iconProps={{ size: 15 }}
              tooltip='功能说明'
              onClick={() => setGuideOpen(true)}
            />
            <IconButton
              size='small'
              icon={RefreshCw}
              iconProps={{ size: 15 }}
              label='刷新当前面板'
              tooltip={refreshHint}
              onClick={() => loadPanel(true)}
            />
          </ActionToolbar>
        }
      />

      <nav className='fallback-panel-grid'>
        {PANEL_ITEMS.map((item) => {
          return (
            <Link
              key={item.key}
              to={`/fallback/${item.key}`}
              className={`fallback-nav-card ${
                activePanel === item.key ? 'active' : ''
              }`}
              style={{ '--panel-accent': item.accent }}
              title={item.description}
            >
              <span className='fallback-nav-icon'>{renderPanelIcon(item)}</span>
              <span className='fallback-nav-content'>
                <span className='fallback-nav-top'>
                  <strong>{item.title}</strong>
                  <span className='fallback-nav-refresh-badge'>
                    每 {formatInterval(PANEL_REFRESH_INTERVALS[item.key])}
                  </span>
                </span>
              </span>
            </Link>
          );
        })}
      </nav>

      <AdminCard className='fallback-content-panel'>
        {renderActivePanel()}
      </AdminCard>
    </div>
  );
};

export default Fallback;
