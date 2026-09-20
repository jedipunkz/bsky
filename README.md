# bsky

A terminal user interface (TUI) for [Bluesky](https://bsky.app), written in Go.

## Features

- Browse your **Home** timeline and **Discover** feed
- Compose and post new posts (up to 300 characters)
- Vim-style keyboard navigation
- Session persistence (no need to log in every time)

## Installation

### Homebrew (recommended)

```bash
brew tap jedipunkz/bsky
brew trust jedipunkz/bsky
brew install bsky
```

Installs a pre-built binary for macOS and Linux, on both arm64 and amd64, so Go
is not needed. `brew trust` is required once per machine: Homebrew does not load
formulae from a third-party tap until the tap is trusted. Upgrade with
`brew upgrade bsky`.

### go install

```bash
go install github.com/jedipunkz/bsky@latest
```

### From source

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

In terminals that speak the Kitty graphics protocol (Kitty, Ghostty, WezTerm, and
multiplexers that forward it such as herdr) the detail view draws pixel-accurate
images automatically. Everywhere else it falls back to half-block characters
(`▀`), which work in any terminal with 24-bit colour support.

Override the automatic choice with `BSKY_KITTY`:

```sh
BSKY_KITTY=1 bsky   # force the Kitty graphics protocol
BSKY_KITTY=0 bsky   # force the half-block renderer
```

Detection is skipped inside tmux and zellij: they do not forward the sequence by
default, so those sessions stay on half-blocks.

For the Sixel protocol, set `BSKY_SIXEL=1`:

```sh
BSKY_SIXEL=1 bsky
```

Sixel stays opt-in and takes precedence over Kitty when set. It cannot be
auto-detected reliably: multiplexers (tmux, zellij, herdr, ...) usually drop
Sixel sequences, which would leave the image area blank.

## Requirements

- Go 1.24+ — only to install with `go install` or to build from source
- A Bluesky account with an [App Password](https://bsky.app/settings/app-passwords)

## License

MIT
