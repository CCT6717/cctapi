import React from 'react';
import { Button, Checkbox, Icon, Input, Message, Modal } from 'semantic-ui-react';

/**
 * KeyModal — overwrite a channel's key with a new value.
 * Pure presentational: GET never returns the existing key, so the only
 * supported action is overwrite. All state/save logic lives in parent.
 *
 * Props:
 *   state: { channelId, newKey, showPlain, saving, error } | null
 *   onChange: (partial) => void
 *   onClose: () => void
 *   onSave: () => void
 */
const KeyModal = ({ state, onChange, onClose, onSave }) => {
  if (!state) return null;
  const { channelId, newKey, showPlain, saving, error } = state;
  return (
    <Modal
      open
      onClose={() => !saving && onClose()}
      size='tiny'
      closeOnEscape={!saving}
      closeOnDimmerClick={!saving}
    >
      <Modal.Header>
        编辑渠道 key {channelId ? `(渠道 #${channelId})` : ''}
      </Modal.Header>
      <Modal.Content>
        <Modal.Description>
          <p style={{ marginBottom: 12, color: '#475569' }}>
            GET 接口不返回原 key，只能用新值覆盖。
          </p>
          <Input
            fluid
            type={showPlain ? 'text' : 'password'}
            placeholder='输入新 key'
            value={newKey || ''}
            onChange={(_, { value }) => onChange({ newKey: value, error: '' })}
            disabled={saving}
          />
          <div style={{ marginTop: 10 }}>
            <Checkbox
              label='显示明文'
              checked={!!showPlain}
              disabled={saving}
              onChange={(_, { checked }) => onChange({ showPlain: checked })}
            />
          </div>
          {error && (
            <Message negative size='small' style={{ marginTop: 12 }}>
              <p>{error}</p>
            </Message>
          )}
        </Modal.Description>
      </Modal.Content>
      <Modal.Actions>
        <Button onClick={onClose} disabled={saving}>
          取消
        </Button>
        <Button
          color='blue'
          loading={saving}
          disabled={saving || !newKey}
          onClick={onSave}
        >
          <Icon name='check' />
          保存
        </Button>
      </Modal.Actions>
    </Modal>
  );
};

export default KeyModal;
