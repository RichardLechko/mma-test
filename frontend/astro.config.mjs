import { defineConfig } from 'astro/config';
import vercel from '@astrojs/vercel/serverless';

// https://astro.build/config
export default defineConfig({
  output: 'server',
  adapter: vercel({
    analytics: true,
    maxDuration: 10, // Increase function timeout to 10 seconds
    includeFiles: ['./dist/**/*'], // Include all necessary files
    excludeFiles: ['./node_modules/**/*'], // Exclude unnecessary files
  }),
  vite: {
    build: {
      // Reduce chunk size to avoid Vercel function size limits
      rollupOptions: {
        output: {
          manualChunks: (id) => {
            if (id.includes('node_modules')) {
              return 'vendor';
            }
          },
        },
      },
    },
  },
  integrations: [partytown()],
});
