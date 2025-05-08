import { defineConfig } from 'vitest/config'; // <-- change from 'vite' to 'vitest/config'
import react from '@vitejs/plugin-react';

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'jsdom',
    setupFiles: './src/vitest.setup.ts'
  }
});
