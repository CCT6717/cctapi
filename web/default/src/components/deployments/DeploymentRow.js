import React, { useState } from 'react';
import { Button, Checkbox, Icon, Input, Label } from 'semantic-ui-react';

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
  onSave,
  onDelete,
}) => {
  const draft = draftDeployments[dep.id] || {};
  const [editBaseUrl, setEditBaseUrl] = useState(null);
  const [editKey, setEditKey] = useState(null);

  const hasChannelChanges = editBaseUrl !== null || editKey !== null;

  const handleSave = () => {
    if (!dep.channel_id) return;
    const payload = {};
    if (editBaseUrl !== null) payload.base_url = editBaseUrl;
    if (editKey !== null) payload.key = editKey;
    onSave(dep.channel_id, payload, () => {
      setEditBaseUrl(null);
      setEditKey(null);
    });
  };

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
          {ownerNames.length > 1 && (
            <Label basic size='mini' color='orange' title={`共享部署：${ownerText}`}>
              共享 {ownerNames.length}
            </Label>
          )}
        </div>

        {/* 模式按钮 - 直接在标题行，无需展开 */}
        <div className='fallback-deploy-mode-btns'>
          <Button
            size='mini'
            className={currentMode === 'fixed' ? 'active-mode' : ''}
            basic={currentMode !== 'fixed'}
            onClick={() => onModeChange('fixed')}
          >
            固定
          </Button>
          <Button
            size='mini'
            className={currentMode === 'quota' ? 'active-mode' : ''}
            basic={currentMode !== 'quota'}
            onClick={() => onModeChange('quota')}
          >
            限额
          </Button>
          <Button
            size='mini'
            className={currentMode === 'error' ? 'active-mode' : ''}
            basic={currentMode !== 'error'}
            onClick={() => onModeChange('error')}
          >
            触发
          </Button>
        </div>

        <div style={{ marginLeft: 'auto', display: 'flex', gap: 6, alignItems: 'center', flexShrink: 0 }}>
          <Button
            type='button'
            size='mini'
            loading={healthTesting}
            disabled={healthTesting || saving}
            onClick={onHealthCheck}
          >
            测试
          </Button>
          {healthResult && (
            <Label basic size='mini' color={healthResult.ok ? 'green' : 'red'}>
              <Icon name={healthResult.ok ? 'check' : 'times'} />
            </Label>
          )}
          {hasChannelChanges && (
            <Button
              type='button'
              size='mini'
              color='blue'
              loading={saving}
              disabled={saving || !dep.channel_id}
              onClick={handleSave}
            >
              保存
            </Button>
          )}
          <Button
            type='button'
            size='mini'
            negative
            disabled={saving}
            onClick={onDelete}
          >
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
          {/* 第一行：4 个数值字段 */}
          <div className='fallback-edit-grid-4'>
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
          </div>

          {/* 第二行：接口地址 + 密钥 */}
          <div className='fallback-edit-grid-2'>
            <div className='fallback-edit-field'>
              <label><Icon name='linkify' /> 接口地址</label>
              <Input
                size='mini'
                fluid
                value={editBaseUrl !== null ? editBaseUrl : dep.base_url || ''}
                placeholder='https://api.example.com/v1'
                onChange={(_, { value }) => setEditBaseUrl(value)}
              />
            </div>
            <div className='fallback-edit-field'>
              <label><Icon name='key' /> 密钥</label>
              <Input
                size='mini'
                fluid
                value={editKey !== null ? editKey : dep.key || ''}
                placeholder='sk-...'
                onChange={(_, { value }) => setEditKey(value)}
              />
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

export default DeploymentRow;
