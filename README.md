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

## Requirements

- Go 1.24+
- A Bluesky account with an [App Password](https://bsky.app/settings/app-passwords)

## License

MIT
