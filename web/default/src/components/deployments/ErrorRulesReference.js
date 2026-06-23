import React from 'react';
import { Icon, Label, Table } from 'semantic-ui-react';

/**
 * ErrorRulesReference — read-only static reference table for D3-b.
 * Documents which upstream errors trigger fallback and which don't.
 * Pure presentational, no props.
 */
const ErrorRulesReference = () => (
  <details style={{ marginTop: 12 }}>
    <summary style={{ cursor: 'pointer', fontSize: 13, color: '#475569', fontWeight: 600 }}>
      <Icon name='info circle' /> 错误触发 fallback 规则（只读参考）
    </summary>
    <Table compact celled size='small' style={{ marginTop: 8 }}>
      <Table.Header>
        <Table.Row>
          <Table.HeaderCell>错误类型</Table.HeaderCell>
          <Table.HeaderCell>触发切换</Table.HeaderCell>
          <Table.HeaderCell>部署状态</Table.HeaderCell>
        </Table.Row>
      </Table.Header>
      <Table.Body>
        <Table.Row>
          <Table.Cell>429 限速</Table.Cell>
          <Table.Cell><Label basic size='mini' color='green'>✓ 切换</Label></Table.Cell>
          <Table.Cell>冷却 60s~300s</Table.Cell>
        </Table.Row>
        <Table.Row>
          <Table.Cell>5xx 服务错误</Table.Cell>
          <Table.Cell><Label basic size='mini' color='green'>✓ 切换</Label></Table.Cell>
          <Table.Cell>冷却 60s~300s</Table.Cell>
        </Table.Row>
        <Table.Row>
          <Table.Cell>402 配额用尽</Table.Cell>
          <Table.Cell><Label basic size='mini' color='green'>✓ 切换</Label></Table.Cell>
          <Table.Cell>标记耗尽到当日末</Table.Cell>
        </Table.Row>
        <Table.Row>
          <Table.Cell>401/403/404</Table.Cell>
          <Table.Cell><Label basic size='mini' color='green'>✓ 切换</Label></Table.Cell>
          <Table.Cell>标记冷却 60s</Table.Cell>
        </Table.Row>
        <Table.Row>
          <Table.Cell>400 参数错误</Table.Cell>
          <Table.Cell><Label basic size='mini' color='grey'>✗ 不切换</Label></Table.Cell>
          <Table.Cell>直接返回错误</Table.Cell>
        </Table.Row>
      </Table.Body>
    </Table>
    <div style={{ fontSize: 12, color: '#94a3b8', marginTop: 6, fontStyle: 'italic' }}>
      注：流式响应已开始写入则不可切换（避免客户端收到半截响应）
    </div>
  </details>
);

export default ErrorRulesReference;
