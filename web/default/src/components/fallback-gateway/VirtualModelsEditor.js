import React from 'react';
import { Checkbox, Icon, Label, Message, Table } from 'semantic-ui-react';

const STRATEGY_LABELS = {
  quality_first: '质量优先',
  cost_first: '成本优先',
  free_first: '免费优先',
};

const POOL_LABELS = {
  paid_high: '付费高质量池',
  cheap: '低成本池',
  local: '本地池',
  free: '免费池',
};

const VM_DISPLAY = {
  'cct/high': { title: '高质量模型', color: 'blue' },
  'cct/low': { title: '低成本模型', color: 'teal' },
};

const VirtualModelsEditor = ({ virtualModels, onChange }) => {
  if (!virtualModels || typeof virtualModels !== 'object') {
    return <Message warning>虚拟模型数据为空或格式错误</Message>;
  }

  const vmKeys = ['cct/high', 'cct/low'].filter((key) => virtualModels[key]);

  const updateVM = (key, field, value) => {
    const updated = {
      ...virtualModels,
      [key]: {
        ...virtualModels[key],
        [field]: value,
      },
    };
    onChange(updated);
  };

  const formatPools = (pools) => {
    const values = Array.isArray(pools) ? pools : [];
    if (values.length === 0) return '-';
    return values.map((pool) => POOL_LABELS[pool] || pool).join(' / ');
  };

  const formatDegrade = (vm) => {
    const labels = [];
    if (vm.allow_degrade_to_low) labels.push('可降级到低成本模型');
    if (vm.allow_degrade_to_free) labels.push('可降级到免费模型');
    return labels.length > 0 ? labels.join('，') : '不降级';
  };

  return (
    <div>
      {vmKeys.length === 0 && (
        <Message info>暂无高质量模型或低成本模型配置。</Message>
      )}
      {vmKeys.length > 0 && (
        <Table compact celled striped>
          <Table.Header>
            <Table.Row>
              <Table.HeaderCell>启用</Table.HeaderCell>
              <Table.HeaderCell>虚拟模型</Table.HeaderCell>
              <Table.HeaderCell>路由池</Table.HeaderCell>
              <Table.HeaderCell>路由策略</Table.HeaderCell>
              <Table.HeaderCell>降级设置</Table.HeaderCell>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {vmKeys.map((key) => {
              const vm = virtualModels[key];
              const display = VM_DISPLAY[key] || { title: key, color: 'grey' };
              return (
                <Table.Row key={key}>
                  <Table.Cell collapsing>
                    <Checkbox
                      toggle
                      checked={!!vm.enabled}
                      onChange={(_, { checked }) => updateVM(key, 'enabled', checked)}
                    />
                  </Table.Cell>
                  <Table.Cell>
                    <strong>{display.title}</strong>
                    <div style={{ marginTop: 4 }}>
                      <Label basic color={display.color} size='small'>
                        <Icon name='server' /> {key}
                      </Label>
                    </div>
                  </Table.Cell>
                  <Table.Cell>{formatPools(vm.pools)}</Table.Cell>
                  <Table.Cell>{STRATEGY_LABELS[vm.strategy] || vm.strategy || '-'}</Table.Cell>
                  <Table.Cell>
                    <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
                      <span>{formatDegrade(vm)}</span>
                      {key === 'cct/high' && (
                        <Checkbox
                          label='允许降级到低成本模型'
                          checked={!!vm.allow_degrade_to_low}
                          onChange={(_, { checked }) => updateVM(key, 'allow_degrade_to_low', checked)}
                        />
                      )}
                      {(key === 'cct/high' || key === 'cct/low') && (
                        <Checkbox
                          label='允许降级到免费模型'
                          checked={!!vm.allow_degrade_to_free}
                          onChange={(_, { checked }) => updateVM(key, 'allow_degrade_to_free', checked)}
                        />
                      )}
                    </div>
                  </Table.Cell>
                </Table.Row>
              );
            })}
          </Table.Body>
        </Table>
      )}
      {virtualModels['cct/free'] && (
        <Message info>
          <Icon name='info circle' />
          免费模型已迁移到「免费模型池」模块管理。
        </Message>
      )}
    </div>
  );
};

export default VirtualModelsEditor;
