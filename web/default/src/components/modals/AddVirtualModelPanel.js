import React from 'react';
import { Button, Dropdown, Icon, Input } from 'semantic-ui-react';

const STRATEGY_OPTIONS = [
  { key: 'quality_first', text: '质量优先', value: 'quality_first' },
  { key: 'cost_first', text: '成本优先', value: 'cost_first' },
  { key: 'free_first', text: '免费优先', value: 'free_first' },
];

/**
 * AddVirtualModelPanel — inline form to create a new virtual model.
 * Pure presentational. All state (newVMName/Strategy/Pool) and submit
 * logic live in the parent.
 *
 * Props:
 *   collapsed: boolean
 *   onExpand: () => void
 *   name, strategy, pool: string
 *   onNameChange, onStrategyChange, onPoolChange: (value) => void
 *   onCancel: () => void
 *   onSubmit: () => void
 *   saving: boolean
 */
const AddVirtualModelPanel = ({
  collapsed,
  onExpand,
  name,
  strategy,
  pool,
  onNameChange,
  onStrategyChange,
  onPoolChange,
  onCancel,
  onSubmit,
  saving,
}) => {
  if (collapsed) {
    return (
      <Button icon labelPosition='left' onClick={onExpand}>
        <Icon name='plus' />
        添加虚拟模型
      </Button>
    );
  }
  return (
    <div style={{ padding: 16, background: '#f8fafc', borderRadius: 8, border: '1px dashed #d9e0ea' }}>
      <div style={{ fontSize: 13, fontWeight: 600, color: '#475569', marginBottom: 12 }}>
        <Icon name='plus circle' /> 新建虚拟模型
      </div>
      <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap', alignItems: 'flex-end' }}>
        <div>
          <label style={{ display: 'block', fontSize: 12, color: '#64748b', marginBottom: 4 }}>名称</label>
          <Input
            size='small'
            placeholder='例: my-model'
            value={name}
            onChange={(_, { value }) => onNameChange(value)}
            style={{ width: 200 }}
          />
        </div>
        <div>
          <label style={{ display: 'block', fontSize: 12, color: '#64748b', marginBottom: 4 }}>路由策略</label>
          <Dropdown
            size='small'
            selection
            value={strategy}
            options={STRATEGY_OPTIONS}
            onChange={(_, { value }) => onStrategyChange(value)}
          />
        </div>
        <div>
          <label style={{ display: 'block', fontSize: 12, color: '#64748b', marginBottom: 4 }}>池名称</label>
          <Input
            size='small'
            placeholder='默认: default'
            value={pool}
            onChange={(_, { value }) => onPoolChange(value)}
            style={{ width: 160 }}
          />
        </div>
        <div style={{ display: 'flex', gap: 8 }}>
          <Button
            color='blue'
            size='small'
            icon
            labelPosition='left'
            loading={saving}
            disabled={!name.trim()}
            onClick={onSubmit}
          >
            <Icon name='check' />
            确认添加
          </Button>
          <Button size='small' onClick={onCancel}>
            取消
          </Button>
        </div>
      </div>
    </div>
  );
};

export default AddVirtualModelPanel;
