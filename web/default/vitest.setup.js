import '@testing-library/jest-dom/vitest';
import { cleanup } from '@testing-library/react';
import { afterEach } from 'vitest';

// Reset the DOM after every test so suites remain isolated.
afterEach(() => {
  cleanup();
});
