import React from 'react';
import { Space } from 'antd';

const joinClassNames = (...values) => values.filter(Boolean).join(' ');

export const ActionToolbar = ({
  align = 'center',
  children,
  className,
  size = 8,
  ...props
}) => (
  <Space
    align={align}
    className={joinClassNames('cct-action-toolbar', className)}
    size={size}
    wrap
    {...props}
  >
    {children}
  </Space>
);

export default ActionToolbar;
