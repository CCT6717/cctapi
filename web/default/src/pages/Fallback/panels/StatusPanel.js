// ============================================================
// StatusPanel.js — Fallback 部署状态面板
// ============================================================

import React from 'react';
import { Button, Dropdown, Icon, Label, Table } from 'semantic-ui-react';
import GatewayStatus from '../../../components/gateway-status/GatewayStatus';
import {
  formatConcurrency,
  formatNumber,
  formatPercent,
  getLevelColor,
  getStatusMeta,
  isQuotaExhaustedRow,
  STATUS_SORT_OPTIONS,
} from '../utils/fallbackHelpers';

const StatusPanel = ({
  statusDisplayRows,
  statusSort,
  setStatusSort,
  actingDeployment,
  runDeploymentAction,
}) => (
  <>
    <GatewayStatus />
    <div className='fallback-content-toolbar'>
      <div>
        <h2>部署状态</h2>
        <span>当前状态、Token 用量和手动操作</span>
      </div>
      <Dropdown
        selection
        compact
        options={STATUS_SORT_OPTIONS}
        value={statusSort}
        onChange={(_, { value }) => setStatusSort(value)}
      />
    </div>
    <div className='fallback-table-wrap'>
      <Table compact celled selectable={false} striped>
        <Table.Header>
          <Table.Row>
            <Table.HeaderCell>部署</Table.HeaderCell>
            <Table.HeaderCell>模型</Table.HeaderCell>
            <Table.HeaderCell>级别</Table.HeaderCell>
            <Table.HeaderCell>用量</Table.HeaderCell>
            <Table.HeaderCell>Token</Table.HeaderCell>
            <Table.HeaderCell>权重</Table.HeaderCell>
            <Table.HeaderCell>并发</Table.HeaderCell>
            <Table.HeaderCell>状态</Table.HeaderCell>
            <Table.HeaderCell>操作</Table.HeaderCell>
          </Table.Row>
        </Table.Header>
        <Table.Body>
          {statusDisplayRows.length === 0 ? (
            <Table.Row>
              <Table.Cell colSpan='9' textAlign='center'>
                暂无 fallback 部署数据
              </Table.Cell>
            </Table.Row>
          ) : (
            statusDisplayRows.map((row) => {
              const statusMeta = getStatusMeta(row);
              return (
                <Table.Row
                  key={row.deployment_id}
                  className={`fallback-deploy-row ${
                    row.alert_type === 'cooldown'
                      ? 'cooling'
                      : isQuotaExhaustedRow(row)
                      ? 'quota-exhausted'
                      : ''
                  }`}
                >
                  <Table.Cell>
                    <strong>{row.deployment_id}</strong>
                    <div className='fallback-muted'>{row.virtual_models}</div>
                  </Table.Cell>
                  <Table.Cell>
                    <span className='fallback-code-text'>
                      {row.real_model}
                    </span>
                  </Table.Cell>
                  <Table.Cell>
                    <Label color={getLevelColor(row.alert_level)}>
                      {row.alert_level || 'normal'}
                    </Label>
                  </Table.Cell>
                  <Table.Cell>{formatPercent(row.usage_percent)}</Table.Cell>
                  <Table.Cell>
                    {formatNumber(row.used_tokens)} /{' '}
                    {formatNumber(row.daily_limit)}
                  </Table.Cell>
                  <Table.Cell>{formatNumber(row.weight || 100)}</Table.Cell>
                  <Table.Cell className='fallback-value-cell'>
                    {formatConcurrency(row)}
                  </Table.Cell>
                  <Table.Cell>
                    <span className='fallback-status-row'>
                      <span
                        className='fallback-status-dot'
                        style={{
                          background:
                            statusMeta.color === 'green'
                              ? '#22c55e'
                              : statusMeta.color === 'orange'
                              ? '#f97316'
                              : statusMeta.color === 'red'
                              ? '#ef4444'
                              : statusMeta.color === 'yellow'
                              ? '#eab308'
                              : '#94a3b8',
                        }}
                      />
                      <span>{statusMeta.text}</span>
                    </span>
                    <div className='fallback-muted'>{statusMeta.note}</div>
                  </Table.Cell>
                  <Table.Cell>
                    <Button.Group size='mini' style={{ minWidth: 120 }}>
                      <Button
                        basic
                        color='orange'
                        title='冷却 5 分钟，不重置额度'
                        loading={
                          actingDeployment ===
                          `${row.deployment_id}:cooldown`
                        }
                        disabled={Boolean(actingDeployment)}
                        onClick={() =>
                          runDeploymentAction(row.deployment_id, 'cooldown')
                        }
                      >
                        <Icon name='pause circle' /> 暂停
                      </Button>
                      <Button
                        basic
                        color='green'
                        title='恢复部署并重置当前周期额度'
                        loading={
                          actingDeployment === `${row.deployment_id}:recover`
                        }
                        disabled={Boolean(actingDeployment)}
                        onClick={() =>
                          runDeploymentAction(row.deployment_id, 'recover')
                        }
                      >
                        <Icon name='undo' /> 恢复
                      </Button>
                    </Button.Group>
                  </Table.Cell>
                </Table.Row>
              );
            })
          )}
        </Table.Body>
      </Table>
    </div>
  </>
);

export default StatusPanel;
