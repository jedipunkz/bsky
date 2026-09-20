// @ts-check
import { defineConfig } from 'astro/config';

// GitHub Pages (project site): https://jedipunkz.github.io/bsky/
export default defineConfig({
  site: 'https://jedipunkz.github.io',
  base: '/bsky',
  trailingSlash: 'ignore',
  build: {
    format: 'directory',
  },
});
