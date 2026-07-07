import React from 'react';
import { Button } from 'antd';
import { AdminCard } from './AdminCard';

const meta = {
  title: 'CCT UI/AdminCard',
  component: AdminCard,
};

export default meta;

export const Basic = {
  render: () => (
    <AdminCard
      title='Free Pool'
      extra={<Button size='small'>Refresh</Button>}
      style={{ maxWidth: 520 }}
    >
      <p style={{ margin: 0, color: '#53627a' }}>
        Compact operational card for admin surfaces.
      </p>
    </AdminCard>
  ),
};
