import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import { federation } from '@module-federation/vite';

export default defineConfig({
  base: process.env.VITE_BASE_PATH || '/modules/payments-module/',
  plugins: [
    react(),
    federation({
      name: 'paymentsModule',
      exposes: {
        './PaymentsPanel': './src/PaymentsPanel.tsx',
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
