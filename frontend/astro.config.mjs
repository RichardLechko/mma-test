import { defineConfig } from 'astro/config';
import vercel from '@astrojs/vercel';
import react from '@astrojs/react';

export default defineConfig({
  output: 'server',
  adapter: vercel(),
  integrations: [react()],
   vite: {
    ssr: {
      noExternal: ['@vercel/analytics'],
    },
  },
});