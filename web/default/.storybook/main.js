const config = {
  stories: ['../src/**/*.stories.@(js|jsx)'],
  addons: ['@storybook/preset-create-react-app', '@storybook/addon-essentials'],
  framework: {
    name: '@storybook/react-webpack5',
    options: {},
  },
  docs: {
    autodocs: 'tag',
  },
};

export default config;
