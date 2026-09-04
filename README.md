# bsky

A terminal user interface (TUI) for [Bluesky](https://bsky.app), written in Go.

## Features

- Browse your **Home** timeline and **Discover** feed
- Compose and post new posts (up to 300 characters)
- Vim-style keyboard navigation
- Session persistence (no need to log in every time)

## Installation

```bash
go install github.com/jedipunkz/bsky@latest
```

Or build from source:

```bash
git clone https://github.com/jedipunkz/bsky.git
cd bsky
go build -o bsky .
```

## Usage

```bash
bsky
```

On first launch, you will be prompted for your Bluesky handle and [App Password](https://bsky.app/settings/app-passwords). The session is saved locally for subsequent runs.

## Keybindings

| Key      | Action                  |
|----------|-------------------------|
| `j`      | Scroll down             |
| `k`      | Scroll up               |
| `h`      | Previous tab            |
| `l`      | Next tab                |
| `c`      | Compose a new post      |
| `r`      | Refresh current feed    |
| `g`      | Jump to top             |
| `G`      | Jump to bottom          |
| `q`      | Quit                    |

### Compose mode

| Key      | Action                  |
|----------|-------------------------|
| `Ctrl+S` | Send post               |
| `Esc`    | Cancel                  |

## Configuration

The config file is stored at `~/.config/bsky/config.yaml`.

### Color Theme

You can set the `theme` field to one of the following values:

| Theme        | Description                        |
|--------------|------------------------------------|
| `tokyonight` | Tokyo Night — dark blue (default)  |
| `kanagawa`   | Kanagawa Wave — warm dark          |
| `solarized`  | Solarized Dark — classic           |
| `catppuccin` | Catppuccin Mocha — pastel dark     |

Example:

```yaml
theme: kanagawa
```

If `theme` is not set or is an unrecognized value, `tokyonight` is used.

### Images

Images in the detail view are drawn with half-block characters (`▀`), which work
in any terminal with 24-bit colour support.

If your terminal supports the Sixel graphics protocol, set `BSKY_SIXEL=1` for
pixel-accurate images:

```sh
BSKY_SIXEL=1 bsky
```

Sixel is opt-in because terminal multiplexers (tmux, zellij, ...) usually drop
Sixel sequences, which would leave the image area blank.

## Requirements

- Go 1.24+
- A Bluesky account with an [App Password](https://bsky.app/settings/app-passwords)

## License

MIT
