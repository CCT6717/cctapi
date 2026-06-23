// ============================================================
// AlertsPanel.js — Fallback 告警历史面板
// ============================================================

import React from 'react';
import { Button, Icon, Label, Table } from 'semantic-ui-react';
import {
  formatNumber,
  formatPercent,
  formatTime,
  getLevelColor,
} from '../utils/fallbackHelpers';

const AlertsPanel = ({
  alertEvents,
  markAllAlertsRead,
}) => (
  <>
    <div className='fallback-content-toolbar'>
      <div>
        <h2>告警历史</h2>
        <span>记录限额、冷却、耗尽和恢复事件。</span>
      </div>
      <div>
        <Button size='small' onClick={markAllAlertsRead}>
          <Icon name='checkmark' /> 全部标为已读
        </Button>
      </div>
    </div>
    <div className='fallback-table-wrap'>
      <Table compact celled striped>
        <Table.Header>
          <Table.Row>
            <Table.HeaderCell>时间</Table.HeaderCell>
            <Table.HeaderCell>部署</Table.HeaderCell>
            <Table.HeaderCell>级别</Table.HeaderCell>
            <Table.HeaderCell>类型</Table.HeaderCell>
            <Table.HeaderCell>Token</Table.HeaderCell>
            <Table.HeaderCell>用量</Table.HeaderCell>
            <Table.HeaderCell>消息</Table.HeaderCell>
          </Table.Row>
        </Table.Header>
        <Table.Body>
          {alertEvents.length === 0 ? (
            <Table.Row>
              <Table.Cell colSpan='7' textAlign='center'>
                暂无告警历史
              </Table.Cell>
            </Table.Row>
          ) : (
            alertEvents.map((event) => (
              <Table.Row key={event.id || `${event.created_at}:${event.deployment_id}`}>
                <Table.Cell>{formatTime(event.created_at)}</Table.Cell>
                <Table.Cell>
                  <a href={`/fallback/status?highlight=${event.deployment_id}`}
                     className='fallback-deployment-link'
                     title='查看部署状态'>
                    <strong>{event.deployment_id}</strong>
                  </a>
                </Table.Cell>
                <Table.Cell>
                  <Label color={getLevelColor(event.level)}>
                    {event.level || '-'}
                  </Label>
                </Table.Cell>
                <Table.Cell>
                  <code>{event.type || '-'}</code>
                </Table.Cell>
                <Table.Cell>
                  {event.daily_limit > 0
                    ? `${formatNumber(event.used_tokens)} / ${formatNumber(
                        event.daily_limit
                      )}`
                    : formatNumber(event.used_tokens)}
                </Table.Cell>
                <Table.Cell>{formatPercent(event.percentage)}</Table.Cell>
                <Table.Cell>{event.message || '-'}</Table.Cell>
              </Table.Row>
            ))
          )}
        </Table.Body>
      </Table>
    </div>
  </>
);

export default AlertsPanel;
