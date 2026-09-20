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

## The hero screenshot

`public/tui.png` is a mock of the TUI, not a capture of a live session: the
posts in it are invented so no real account appears on the site. It is rendered
from `tools/tui-mock.html` — a standalone page using the same Catppuccin Mocha
colors — with headless Chromium at a device pixel ratio of 2:

```js
// npm i -D playwright  (Chromium only)
const page = await browser.newPage({
  viewport: { width: 920, height: 810 },
  deviceScaleFactor: 2,
});
await page.goto('file:///abs/path/to/site/tools/tui-mock.html');
await page.screenshot({ path: 'public/tui.png' });
```

Edit the HTML and re-render when the TUI's layout changes; keep the viewport
and the `width`/`height` attributes on the `<img>` in `src/pages/index.astro`
in sync.

## Deployment

`.github/workflows/pages.yaml` builds this directory and deploys `dist/` to
GitHub Pages on every push to `main` that touches `site/`. Pull requests run the
build only. The repository must have **Settings → Pages → Source** set to
**GitHub Actions**.

Because the site is served from a project path, `astro.config.mjs` sets
`site: 'https://jedipunkz.github.io'` and `base: '/bsky'`. Use
`import.meta.env.BASE_URL` for internal links so they keep working under that
base.
