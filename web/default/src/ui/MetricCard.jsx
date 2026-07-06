import React from 'react';
import { Card } from 'antd';

export const MetricCard = ({
  caption,
  className,
  icon,
  title,
  value,
  ...props
}) => (
  <Card className={`cct-admin-card cct-metric-card ${className || ''}`} {...props}>
    <div className='cct-metric-card-head'>
      <span className='cct-metric-card-title'>{title}</span>
      {icon && <span className='cct-metric-card-icon'>{icon}</span>}
    </div>
    <div className='cct-metric-card-value'>{value}</div>
    {caption && <div className='cct-metric-card-caption'>{caption}</div>}
  </Card>
);

export default MetricCard;
