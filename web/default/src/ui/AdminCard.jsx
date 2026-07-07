import React from 'react';
import { Card } from 'antd';

const joinClassNames = (...values) => values.filter(Boolean).join(' ');

export const AdminCard = ({ children, className, ...props }) => (
  <Card className={joinClassNames('cct-admin-card', className)} {...props}>
    {children}
  </Card>
);

export default AdminCard;
