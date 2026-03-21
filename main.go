package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jedipunkz/bsky/internal/api"
	"github.com/jedipunkz/bsky/internal/config"
	"github.com/jedipunkz/bsky/internal/ui"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	client := api.NewClient()

	// Use saved session if available
	if cfg.AccessJWT != "" {
		client.SetSession(cfg.AccessJWT, cfg.RefreshJWT, cfg.DID, cfg.Handle)
	} else {
		// Interactive login
		if err := login(client, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Login failed: %v\n", err)
			os.Exit(1)
		}
	}

	model := ui.New(client, cfg.Theme)
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func login(client *api.Client, cfg *config.Config) error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Handle (e.g. user.bsky.social): ")
	handle, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	handle = strings.TrimSpace(handle)

	fmt.Print("Password (App Password): ")
	password, err := readPassword()
	if err != nil {
		return err
	}
	fmt.Println()

	sess, err := client.CreateSession(handle, password)
	if err != nil {
		return err
	}

	cfg.Handle = sess.Handle
	cfg.AccessJWT = sess.AccessJwt
	cfg.RefreshJWT = sess.RefreshJwt
	cfg.DID = sess.DID

	if err := config.Save(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not save session: %v\n", err)
	}

	return nil
}

func readPassword() (string, error) {
	var oldState syscall.Termios
	fd := int(os.Stdin.Fd())
	if _, _, errno := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(fd),
		syscall.TCGETS, uintptr(unsafe.Pointer(&oldState)), 0, 0, 0); errno != 0 {
		reader := bufio.NewReader(os.Stdin)
		pw, err := reader.ReadString('\n')
		return strings.TrimSpace(pw), err
	}

	newState := oldState
	newState.Lflag &^= syscall.ECHO
	_, _, _ = syscall.Syscall6(syscall.SYS_IOCTL, uintptr(fd),
		syscall.TCSETS, uintptr(unsafe.Pointer(&newState)), 0, 0, 0)
	defer func() {
		_, _, _ = syscall.Syscall6(syscall.SYS_IOCTL, uintptr(fd),
			syscall.TCSETS, uintptr(unsafe.Pointer(&oldState)), 0, 0, 0)
	}()

	reader := bufio.NewReader(os.Stdin)
	pw, err := reader.ReadString('\n')
	return strings.TrimSpace(pw), err
}
