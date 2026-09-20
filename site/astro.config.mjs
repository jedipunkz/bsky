// @ts-check
import { defineConfig } from 'astro/config';
import sitemap from '@astrojs/sitemap';

// GitHub Pages project site, served on the jedipunkz.rocks custom domain:
// https://jedipunkz.rocks/bsky/ (github.io 301-redirects here).
export default defineConfig({
  site: 'https://jedipunkz.rocks',
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
