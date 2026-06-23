// ============================================================
// LogsPanel.js — Fallback 切换快照面板
// ============================================================

import React from 'react';
import { Label, Message, Table } from 'semantic-ui-react';
import { formatTime, translateFallbackReason } from '../utils/fallbackHelpers';

const LogsPanel = ({ switchEvents }) => (
  <>
    <div className='fallback-content-toolbar'>
      <div>
        <h2>回退事件日志</h2>
        <span>记录最近的部署切换、原因和请求耗时。</span>
      </div>
    </div>
    <Message info className='fallback-log-scope-note'>
      这里展示的是 fallback 业务事件：只有发生部署切换时才会记录。程序启动、数据库、Redis
      等系统运行日志请看服务日志文件。
    </Message>
    <div className='fallback-table-wrap'>
      <Table compact celled striped>
        <Table.Header>
          <Table.Row>
            <Table.HeaderCell>时间</Table.HeaderCell>
            <Table.HeaderCell>虚拟模型</Table.HeaderCell>
            <Table.HeaderCell>切换</Table.HeaderCell>
            <Table.HeaderCell>原因</Table.HeaderCell>
            <Table.HeaderCell>状态码</Table.HeaderCell>
            <Table.HeaderCell>耗时</Table.HeaderCell>
            <Table.HeaderCell>请求 ID</Table.HeaderCell>
          </Table.Row>
        </Table.Header>
        <Table.Body>
          {switchEvents.length === 0 ? (
            <Table.Row>
              <Table.Cell colSpan='7' textAlign='center'>
                暂无回退切换事件
              </Table.Cell>
            </Table.Row>
          ) : (
            switchEvents.map((event) => (
              <Table.Row key={event.id || `${event.created_at}:${event.request_id}`}>
                <Table.Cell>{formatTime(event.created_at)}</Table.Cell>
                <Table.Cell>
                  <strong>{event.virtual_model || '-'}</strong>
                </Table.Cell>
                <Table.Cell>
                  <strong>{event.from_deployment || '-'}</strong>
                  <span className='fallback-arrow'>-&gt;</span>
                  <strong>{event.to_deployment || '-'}</strong>
                </Table.Cell>
                <Table.Cell>{translateFallbackReason(event.reason)}</Table.Cell>
                <Table.Cell>
                  <Label
                    color={
                      event.status_code >= 500
                        ? 'red'
                        : event.status_code >= 400
                        ? 'yellow'
                        : 'green'
                    }
                  >
                    {event.status_code || '-'}
                  </Label>
                </Table.Cell>
                <Table.Cell>
                  {event.duration_ms > 0 ? `${event.duration_ms}ms` : '-'}
                </Table.Cell>
                <Table.Cell>
                  <code>{event.request_id || '-'}</code>
                </Table.Cell>
              </Table.Row>
            ))
          )}
        </Table.Body>
      </Table>
    </div>
  </>
);

export default LogsPanel;
