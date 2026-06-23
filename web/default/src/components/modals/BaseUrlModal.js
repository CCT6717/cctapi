import React from 'react';
import { Button, Icon, Input, Message, Modal } from 'semantic-ui-react';

/**
 * BaseUrlModal — edit a channel's base_url.
 * Pure presentational: all state and save logic lives in the parent.
 *
 * Props:
 *   state: { channelId, baseUrl, saving, error } | null
 *   onChange: (partial) => void   — merge partial into state
 *   onClose: () => void
 *   onSave: () => void
 */
const BaseUrlModal = ({ state, onChange, onClose, onSave }) => {
  if (!state) return null;
  const { channelId, baseUrl, saving, error } = state;
  return (
    <Modal
      open
      onClose={() => !saving && onClose()}
      size='small'
      closeOnEscape={!saving}
      closeOnDimmerClick={!saving}
    >
      <Modal.Header>
        编辑 base_url {channelId ? `(渠道 #${channelId})` : ''}
      </Modal.Header>
      <Modal.Content>
        <Modal.Description>
          <p style={{ marginBottom: 12, color: '#475569' }}>
            修改该渠道的 <code>base_url</code>，保存后立即生效。
            <br />
            <span style={{ color: '#94a3b8', fontSize: 12 }}>
              末尾斜杠会被自动去除（统一格式）。
            </span>
          </p>
          <Input
            fluid
            placeholder='https://api.example.com/v1'
            value={baseUrl || ''}
            onChange={(_, { value }) => onChange({ baseUrl: value, error: '' })}
            disabled={saving}
          />
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
        <Button color='blue' loading={saving} disabled={saving} onClick={onSave}>
          <Icon name='check' />
          保存
        </Button>
      </Modal.Actions>
    </Modal>
  );
};

export default BaseUrlModal;
