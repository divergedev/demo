import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import { federation } from '@module-federation/vite';

export default defineConfig({
  plugins: [
    react(),
    federation({
      name: 'shell',
      remotes: {
        paymentsModule: {
          type: 'module',
          name: 'paymentsModule',
          entry: '/modules/payments-module/remoteEntry.js',
          entryGlobalName: 'paymentsModule',
        },
      },
      shared: {
        react: {
          singleton: true,
        },
        'react-dom': {
          singleton: true,
        },
      },
    }),
  ],
  build: {
    outDir: '../dist',
    emptyOutDir: true,
    target: 'chrome89',
  },
});
