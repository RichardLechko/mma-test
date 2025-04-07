import { defineConfig } from 'astro/config';

// https://astro.build/config
export default defineConfig({
  output: 'server', // Enable SSR
  // other settings
  devToolbar: {
    enabled: false
  }
});