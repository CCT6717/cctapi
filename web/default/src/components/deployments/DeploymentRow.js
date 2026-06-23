import React, { useState } from 'react';
import { Button, Checkbox, Icon, Input, Label } from 'semantic-ui-react';

/**
 * DeploymentRow — one deployment entry inside a VM, with expandable details.
 * Pure presentational. All state and callbacks come from the parent.
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
  onTestAll,
  onDelete,
}) => {
  const draft = draftDeployments[dep.id] || {};
  const [showKey, setShowKey] = useState(false);

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
        <div style={{ marginLeft: 'auto', display: 'flex', gap: 8, alignItems: 'center' }}>
          <Button
            size='mini'
            basic
            color='blue'
            icon
            labelPosition='left'
            disabled={saving}
            onClick={onTestAll}
          >
            <Icon name='heartbeat' />
            测试全部
          </Button>
          <Button
            size='mini'
            icon
            labelPosition='left'
            loading={healthTesting}
            disabled={healthTesting || saving}
            onClick={onHealthCheck}
          >
            <Icon name='heartbeat' />
            测试
          </Button>
          {healthResult && (
            <Label basic size='mini' color={healthResult.ok ? 'green' : 'red'}>
              <Icon name={healthResult.ok ? 'check' : 'times'} />
              {healthResult.text}
            </Label>
          )}
          <Button
            size='mini'
            negative
            icon
            labelPosition='left'
            disabled={saving}
            onClick={onDelete}
          >
            <Icon name='trash' />
            删除
          </Button>
          <Checkbox
            toggle
            checked={draft.enabled !== undefined ? draft.enabled : dep.enabled !== false}
            onChange={(_, { checked }) => onDraftField('enabled', checked)}
          />
        </div>
      </div>

      <div className='fallback-state-note'>
        {statusMeta.detail}
      </div>

      {expanded && (
        <div className='fallback-deployment-details'>
          {/* 部署模式按钮 - 第一行 */}
          <div className='fallback-edit-mode-row'>
            <Button
              size='mini'
              color={currentMode === 'fixed' ? 'purple' : undefined}
              basic={currentMode !== 'fixed'}
              onClick={() => onModeChange('fixed')}
            >
              固定模式
            </Button>
            <Button
              size='mini'
              color={currentMode === 'quota' ? 'orange' : undefined}
              basic={currentMode !== 'quota'}
              onClick={() => onModeChange('quota')}
            >
              限额模式
            </Button>
            <Button
              size='mini'
              color={currentMode === 'error' ? 'green' : undefined}
              basic={currentMode !== 'error'}
              onClick={() => onModeChange('error')}
            >
              触发模式
            </Button>
          </div>

          {/* 横向字段网格 */}
          <div className='fallback-edit-grid'>
            <div className='fallback-edit-field'>
              <label>渠道 ID</label>
              <span className='fallback-edit-value'>{dep.channel_id || '-'}</span>
            </div>
            <div className='fallback-edit-field'>
              <label>优先级</label>
              <Input
                type='number'
                size='mini'
                value={draft.priority !== undefined ? draft.priority : dep.priority ?? 0}
                onChange={(_, { value }) => onDraftField('priority', value)}
              />
            </div>
            <div className='fallback-edit-field'>
              <label>权重</label>
              <Input
                type='number'
                size='mini'
                value={draft.weight !== undefined ? draft.weight : dep.weight ?? 100}
                onChange={(_, { value }) => onDraftField('weight', value)}
              />
            </div>
            <div className='fallback-edit-field'>
              <label>每日 Token 限额</label>
              <Input
                type='number'
                size='mini'
                placeholder='0 = 无限制'
                value={draft.daily_limit_tokens ?? dep.daily_limit_tokens ?? 0}
                onChange={(_, { value }) => onDraftField('daily_limit_tokens', value)}
              />
            </div>
            <div className='fallback-edit-field fallback-edit-field-wide'>
              <label>接口地址</label>
              <div className='fallback-edit-value-row'>
                <span className='fallback-edit-value fallback-edit-url'>
                  {dep.base_url || '-'}
                </span>
              </div>
            </div>
            <div className='fallback-edit-field fallback-edit-field-wide'>
              <label>密钥</label>
              <div className='fallback-edit-value-row'>
                <span className='fallback-edit-value fallback-edit-key'>
                  {dep.key ? (showKey ? dep.key : '••••••••') : '-'}
                </span>
                <Button
                  size='mini'
                  basic
                  icon
                  onClick={() => setShowKey(!showKey)}
                  title={showKey ? '隐藏密钥' : '显示密钥'}
                >
                  <Icon name={showKey ? 'eye slash' : 'eye'} />
                </Button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

export default DeploymentRow;
