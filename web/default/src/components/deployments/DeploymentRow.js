import React from 'react';
import { Button, Checkbox, Icon, Input, Label, Table } from 'semantic-ui-react';
import ErrorRulesReference from './ErrorRulesReference';

/**
 * DeploymentRow — one deployment entry inside a VM, with expandable details.
 * Pure presentational. All state and callbacks come from the parent.
 *
 * Props:
 *   dep: deployment object (id, real_model, channel_id, daily_limit_tokens, ...)
 *   orderIndex: number
 *   expanded: boolean
 *   highlighted: boolean
 *   statusMeta: { label, color, detail }
 *   ownerNames: string[]
 *   ownerText: string
 *   vmKey: string
 *   draftDeployments: object — current draft edits
 *   deploymentMode: object — { [depId]: 'fixed'|'quota'|'error' }
 *   currentMode: 'fixed'|'quota'|'error' — resolved mode for this dep
 *   healthTesting: boolean
 *   healthResult: { ok, text } | null
 *   saving: boolean
 *   onToggle: () => void
 *   onDraftField: (field, value) => void
 *   onModeChange: (mode) => void
 *   onHealthCheck: () => void
 *   onEditBaseUrl: () => void
 *   onEditKey: () => void
 *   onDelete: () => void
 */
const DeploymentRow = ({
  dep,
  orderIndex,
  expanded,
  highlighted,
  statusMeta,
  ownerNames,
  ownerText,
  vmKey,
  draftDeployments,
  currentMode,
  healthTesting,
  healthResult,
  saving,
  onToggle,
  onDraftField,
  onModeChange,
  onHealthCheck,
  onEditBaseUrl,
  onEditKey,
  onDelete,
}) => {
  const draft = draftDeployments[dep.id] || {};
  return (
    <div className={`fallback-deployment-panel ${highlighted ? 'fallback-highlight' : ''}`}>
      <div className='fallback-deployment-heading'>
        <Button
          type='button'
          basic
          circular
          className='fallback-collapse-button'
          icon={expanded ? 'angle down' : 'angle right'}
          onClick={onToggle}
        />
        <div className='fallback-deployment-name'>
          {dep.real_model || '未命名真实模型'}
          <Label basic size='mini' color='teal'>
            {`顺序 #${orderIndex + 1}`}
          </Label>
          <Label basic size='mini' color={statusMeta.color}>
            {statusMeta.label}
          </Label>
          <Label basic size='mini' color={dep.daily_limit_tokens > 0 ? 'teal' : 'blue'}>
            {dep.daily_limit_tokens > 0 ? '限额 ' + (dep.daily_limit_tokens || 0).toLocaleString() : '触发模式'}
          </Label>
          {ownerNames.length > 1 && (
            <Label basic size='mini' color='orange' title={`共享部署：${ownerText}`}>
              共享 {ownerNames.length}
            </Label>
          )}
        </div>
      </div>

      <div className='fallback-state-note'>
        {statusMeta.detail}
      </div>

      {expanded && (
        <div className='fallback-deployment-details'>
          <Table compact celled size='small'>
            <Table.Body>
              <Table.Row>
                <Table.Cell>渠道 ID</Table.Cell>
                <Table.Cell>{dep.channel_id || '-'}</Table.Cell>
              </Table.Row>
              <Table.Row>
                <Table.Cell>启用</Table.Cell>
                <Table.Cell>
                  <Checkbox
                    toggle
                    checked={draft.enabled !== undefined ? draft.enabled : dep.enabled !== false}
                    onChange={(_, { checked }) => onDraftField('enabled', checked)}
                  />
                </Table.Cell>
              </Table.Row>
              <Table.Row>
                <Table.Cell>优先级</Table.Cell>
                <Table.Cell>
                  <Input
                    type='number'
                    size='mini'
                    style={{ maxWidth: 100 }}
                    value={draft.priority !== undefined ? draft.priority : dep.priority ?? 0}
                    onChange={(_, { value }) => onDraftField('priority', value)}
                  />
                </Table.Cell>
              </Table.Row>
              <Table.Row>
                <Table.Cell>权重</Table.Cell>
                <Table.Cell>
                  <Input
                    type='number'
                    size='mini'
                    style={{ maxWidth: 100 }}
                    value={draft.weight !== undefined ? draft.weight : dep.weight ?? 100}
                    onChange={(_, { value }) => onDraftField('weight', value)}
                  />
                </Table.Cell>
              </Table.Row>
              <Table.Row>
                <Table.Cell>部署模式</Table.Cell>
                <Table.Cell>
                  <Button.Group size='mini'>
                    <Button
                      color={currentMode === 'fixed' ? 'blue' : undefined}
                      onClick={() => onModeChange('fixed')}
                    >
                      固定模式
                    </Button>
                    <Button
                      color={currentMode === 'quota' ? 'orange' : undefined}
                      onClick={() => onModeChange('quota')}
                    >
                      限额模式
                    </Button>
                    <Button
                      color={currentMode === 'error' ? 'green' : undefined}
                      onClick={() => onModeChange('error')}
                    >
                      触发模式
                    </Button>
                  </Button.Group>
                </Table.Cell>
              </Table.Row>
              <Table.Row>
                <Table.Cell>每日 Token 限额</Table.Cell>
                <Table.Cell>
                  <Input
                    type='number'
                    size='mini'
                    style={{ maxWidth: 140 }}
                    placeholder='0 = 无限制'
                    value={draft.daily_limit_tokens ?? dep.daily_limit_tokens ?? 0}
                    onChange={(_, { value }) => onDraftField('daily_limit_tokens', value)}
                  />
                </Table.Cell>
              </Table.Row>
              <Table.Row>
                <Table.Cell>软限比例</Table.Cell>
                <Table.Cell>
                  <Input
                    type='number'
                    size='mini'
                    step='0.01'
                    min='0'
                    max='1'
                    style={{ maxWidth: 100 }}
                    placeholder='默认 0.95'
                    value={draft.soft_limit_ratio ?? dep.soft_limit_ratio ?? 0.95}
                    onChange={(_, { value }) => onDraftField('soft_limit_ratio', value)}
                  />
                </Table.Cell>
              </Table.Row>
              <Table.Row>
                <Table.Cell>硬限比例</Table.Cell>
                <Table.Cell>
                  <Input
                    type='number'
                    size='mini'
                    step='0.01'
                    min='0'
                    max='1'
                    style={{ maxWidth: 100 }}
                    placeholder='默认 1.0'
                    value={draft.hard_limit_ratio ?? dep.hard_limit_ratio ?? 1.0}
                    onChange={(_, { value }) => onDraftField('hard_limit_ratio', value)}
                  />
                </Table.Cell>
              </Table.Row>
              <Table.Row>
                <Table.Cell>操作</Table.Cell>
                <Table.Cell>
                  <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
                    <Button
                      size='mini'
                      icon
                      labelPosition='left'
                      loading={healthTesting}
                      disabled={healthTesting || saving}
                      onClick={onHealthCheck}
                    >
                      <Icon name='heartbeat' />
                      连通性测试
                    </Button>
                    {healthResult && (
                      <Label basic size='mini' color={healthResult.ok ? 'green' : 'red'}>
                        <Icon name={healthResult.ok ? 'check' : 'times'} />
                        {healthResult.text}
                      </Label>
                    )}
                    <Button
                      size='mini'
                      color='blue'
                      icon
                      labelPosition='left'
                      disabled={!dep.channel_id || saving}
                      onClick={onEditBaseUrl}
                      title={!dep.channel_id ? '该部署未绑定渠道' : '编辑该渠道的 base_url'}
                    >
                      <Icon name='linkify' />
                      编辑 base_url
                    </Button>
                    <Button
                      size='mini'
                      icon
                      labelPosition='left'
                      disabled={!dep.channel_id || saving}
                      onClick={onEditKey}
                      title={!dep.channel_id ? '该部署未绑定渠道' : '用新值覆盖该渠道的 key (原值不可查)'}
                    >
                      <Icon name='key' />
                      编辑 key
                    </Button>
                    <Button
                      size='mini'
                      negative
                      icon
                      labelPosition='left'
                      loading={saving}
                      onClick={onDelete}
                    >
                      <Icon name='trash' />
                      删除此部署
                    </Button>
                  </div>
                </Table.Cell>
              </Table.Row>
            </Table.Body>
          </Table>

          <ErrorRulesReference />
        </div>
      )}
    </div>
  );
};

export default DeploymentRow;
