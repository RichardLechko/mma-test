import { defineConfig } from 'astro/config';
import vercel from "@astrojs/vercel";

import partytown from '@astrojs/partytown';

// https://astro.build/config
export default defineConfig({
  output: 'server',
  adapter: vercel(),

  devToolbar: {
    enabled: false
  },

  integrations: [partytown()]
});