import React from 'react';
import { Tag } from 'antd';

const STATUS_COLORS = {
  default: 'default',
  healthy: 'success',
  success: 'success',
  warning: 'warning',
  rate_limited: 'warning',
  error: 'error',
  invalid: 'error',
  disabled: 'default',
  info: 'processing',
};

export const StatusTag = ({
  children,
  className,
  status = 'default',
  showDot = true,
  ...props
}) => {
  const color = STATUS_COLORS[status] || STATUS_COLORS.default;

  return (
    <Tag className={`cct-status-tag ${className || ''}`} color={color} {...props}>
      {showDot && <span className='cct-status-tag-dot' />}
      {children}
    </Tag>
  );
};

export default StatusTag;
