# bsky site

Landing page for [bsky](https://github.com/jedipunkz/bsky), built with
[Astro](https://astro.build) and themed with the
[Catppuccin Mocha](https://github.com/catppuccin/catppuccin) palette.

Published at <https://jedipunkz.github.io/bsky/>.

## Commands

| Command         | Action                                      |
|-----------------|---------------------------------------------|
| `npm install`   | Install dependencies                        |
| `npm run dev`   | Dev server at <http://localhost:4321/bsky>  |
| `npm run build` | Build the static site into `dist/`          |
| `npm run preview` | Serve the built site locally              |
| `npm run check` | Type-check the Astro components             |

## Deployment

`.github/workflows/pages.yaml` builds this directory and deploys `dist/` to
GitHub Pages on every push to `main` that touches `site/`. Pull requests run the
build only. The repository must have **Settings → Pages → Source** set to
**GitHub Actions**.

Because the site is served from a project path, `astro.config.mjs` sets
`site: 'https://jedipunkz.github.io'` and `base: '/bsky'`. Use
`import.meta.env.BASE_URL` for internal links so they keep working under that
base.
