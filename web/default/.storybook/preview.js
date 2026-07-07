import React from 'react';
import { ConfigProvider } from 'antd';
import 'antd/dist/reset.css';
import '../src/ui/ui.css';
import { cctAntdTheme } from '../src/ui/theme';

const preview = {
  decorators: [
    (Story) => (
      <ConfigProvider theme={cctAntdTheme}>
        <div style={{ padding: 24, background: '#f8fafc', minHeight: '100vh' }}>
          <Story />
        </div>
      </ConfigProvider>
    ),
  ],
  parameters: {
    controls: {
      matchers: {
        color: /(background|color)$/i,
        date: /Date$/i,
      },
    },
  },
};

export default preview;
