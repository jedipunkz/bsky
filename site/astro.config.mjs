// @ts-check
import { defineConfig } from 'astro/config';
import sitemap from '@astrojs/sitemap';

// GitHub Pages (project site): https://jedipunkz.github.io/bsky/
export default defineConfig({
  site: 'https://jedipunkz.github.io',
  base: '/bsky',
  trailingSlash: 'ignore',
  build: {
    format: 'directory',
  },
  integrations: [
    sitemap({
      // trailingSlash: 'ignore' emits both /bsky and /bsky/ for the same page;
      // keep only the slashed form the canonical link points at, which also
      // drops the 404 route.
      filter: (page) => page.endsWith('/'),
    }),
  ],
});
