import React, { useState } from 'react';
import { Checkbox, Icon, Message } from 'semantic-ui-react';

const VM_LABELS = {
  'cct/high': '高质量模型',
  'cct/low':  '低成本模型',
  'cct/free': '免费模型',
};
const POOL_CN = { paid_high: '付费高质量池', cheap: '低成本池', free: '免费池', local: '本地池' };
const STRATEGY_CN = { quality_first: '质量优先', cost_first: '成本优先', free_first: '免费优先' };

const VirtualModelsEditor = ({ virtualModels, deployments, onChange }) => {
  const [expanded, setExpanded] = useState({});

  if (!virtualModels || typeof virtualModels !== 'object') return <Message warning>虚拟模型数据为空或格式错误</Message>;
  const vmKeys = Object.keys(virtualModels);
  if (vmKeys.length === 0) return <Message info>暂无虚拟模型配置。</Message>;

  const toggleExpand = (k) => setExpanded((p) => ({ ...p, [k]: !p[k] }));
  const updateVM = (key, field, value) => {
    onChange({ ...virtualModels, [key]: { ...virtualModels[key], [field]: value } });
  };

  // Find deployments associated with a virtual model
  const getRelatedDeps = (vmKey) => {
    if (!deployments || typeof deployments !== 'object') return [];
    const vm = virtualModels[vmKey];
    const pools = Array.isArray(vm?.pools) ? vm.pools : [];
    return Object.keys(deployments).filter((k) => pools.includes(deployments[k].pool));
  };

  return (
    <div style={{ overflowX: 'auto' }}>
      <table className='gateway-table'>
        <thead>
          <tr>
            <th style={{ width: 30 }}></th>
            <th>启用</th>
            <th>虚拟模型</th>
            <th>路由池</th>
            <th>路由策略</th>
            <th>降级设置</th>
          </tr>
        </thead>
        <tbody>
          {vmKeys.map((key) => {
            const vm = virtualModels[key];
            const isOpen = !!expanded[key];
            const label = VM_LABELS[key] || key;
            const pools = Array.isArray(vm.pools) ? vm.pools.map((p) => POOL_CN[p] || p).join('、') : '-';
            const strategy = STRATEGY_CN[vm.strategy] || vm.strategy || '-';
            const degradeLabels = [];
            if (vm.allow_degrade_to_low) degradeLabels.push('→ 低成本模型');
            if (vm.allow_degrade_to_free) degradeLabels.push('→ 免费模型');
            const showDegradeLow = key === 'cct/high';
            const showDegradeFree = key === 'cct/high' || key === 'cct/low';

            return (
              <React.Fragment key={key}>
                <tr>
                  <td>
                    <span className='gateway-expand-toggle' onClick={() => toggleExpand(key)}>
                      <Icon name={isOpen ? 'chevron down' : 'chevron right'} />
                    </span>
                  </td>
                  <td>
                    <Checkbox toggle checked={!!vm.enabled} onChange={(_, { checked }) => updateVM(key, 'enabled', checked)} />
                  </td>
                  <td><strong>{label}</strong><div className='gateway-muted' style={{ fontSize: 11 }}>{key}</div></td>
                  <td><span style={{ fontSize: 12 }}>{pools}</span></td>
                  <td><span style={{ fontSize: 12 }}>{strategy}</span></td>
                  <td>
                    <span className='gateway-muted' style={{ fontSize: 12 }}>
                      {degradeLabels.length > 0 ? degradeLabels.join('  ') : '-'}
                    </span>
                  </td>
                </tr>
                {isOpen && (
                  <tr>
                    <td colSpan='6' className='gateway-row-expanded'>
                      <div className='gateway-detail-grid'>
                        <div className='gateway-detail-item'>
                          <span className='detail-label'>模型 ID</span>
                          <span className='gateway-mono'>{key}</span>
                        </div>
                        <div className='gateway-detail-item'>
                          <span className='detail-label'>路由池（只读）</span>
                          <span>{pools}</span>
                        </div>
                        <div className='gateway-detail-item'>
                          <span className='detail-label'>路由策略（只读）</span>
                          <span>{strategy}</span>
                        </div>
                        {showDegradeLow && (
                          <div className='gateway-detail-item'>
                            <span className='detail-label'>允许降级到低成本模型</span>
                            <Checkbox checked={!!vm.allow_degrade_to_low} onChange={(_, { checked }) => updateVM(key, 'allow_degrade_to_low', checked)} />
                          </div>
                        )}
                        {showDegradeFree && (
                          <div className='gateway-detail-item'>
                            <span className='detail-label'>允许降级到免费模型</span>
                            <Checkbox checked={!!vm.allow_degrade_to_free} onChange={(_, { checked }) => updateVM(key, 'allow_degrade_to_free', checked)} />
                          </div>
                        )}
                      </div>
                      {/* Related deployments */}
                      {(() => {
                        const deps = getRelatedDeps(key);
                        if (deps.length === 0) return null;
                        return (
                          <div style={{ marginTop: 8, paddingTop: 8, borderTop: '1px solid #f1f5f9' }}>
                            <span className='detail-label' style={{ display: 'block', marginBottom: 4 }}>关联部署（{deps.length} 个）</span>
                            <div style={{ display: 'flex', flexWrap: 'wrap', gap: 4 }}>
                              {deps.map((d) => (
                                <span key={d} className='gateway-badge info' style={{ fontSize: 11 }}>{d}</span>
                              ))}
                            </div>
                          </div>
                        );
                      })()}
                    </td>
                  </tr>
                )}
              </React.Fragment>
            );
          })}
        </tbody>
      </table>
    </div>
  );
};

export default VirtualModelsEditor;
