package main

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/nanaki-93/mini-orca/v2/desktop/webapp/internal/assets"
	"github.com/nanaki-93/mini-orca/v2/desktop/webapp/internal/bridge"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type Desktop struct {
	ctx    context.Context
	client *bridge.Client
	mu     sync.Mutex
	dirty  bool
}

func (d *Desktop) Request(id, method, path, body string) (bridge.Response, error) {
	return d.client.Request(d.ctx, id, method, path, body)
}

func (d *Desktop) Cancel(id string) { d.client.Cancel(id) }

func (d *Desktop) ChooseDirectory() (string, error) {
	return runtime.OpenDirectoryDialog(d.ctx, runtime.OpenDialogOptions{Title: "Open project", CanCreateDirectories: false})
}

func (d *Desktop) SetUnsavedDraft(dirty bool) {
	d.mu.Lock()
	d.dirty = dirty
	d.mu.Unlock()
}

func (d *Desktop) OpenTerminal(root string, cols, rows int) (bridge.TerminalUpdate, error) {
	return d.client.OpenTerminal(root, cols, rows)
}

func (d *Desktop) ReadTerminal(id string, cursor int64) (bridge.TerminalUpdate, error) {
	return d.client.Terminals().Read(id, cursor)
}

func (d *Desktop) WriteTerminal(id, data string) error { return d.client.Terminals().Write(id, data) }

func (d *Desktop) ResizeTerminal(id string, cols, rows int) error {
	return d.client.Terminals().Resize(id, cols, rows)
}

func (d *Desktop) CloseTerminal(id string) error { return d.client.Terminals().Close(id) }

func (d *Desktop) beforeClose(ctx context.Context) bool {
	d.mu.Lock()
	dirty := d.dirty
	d.mu.Unlock()
	if dirty {
		answer, err := runtime.MessageDialog(ctx, runtime.MessageDialogOptions{Type: runtime.QuestionDialog, Title: "Close Mini-Orca?", Message: "Your current draft will be discarded.", Buttons: []string{"Keep working", "Discard and close"}, DefaultButton: "Keep working", CancelButton: "Keep working"})
		if err != nil || answer != "Discard and close" {
			return true
		}
	}
	if err := d.client.Close(); err != nil {
		_, _ = runtime.MessageDialog(ctx, runtime.MessageDialogOptions{Type: runtime.ErrorDialog, Title: "Terminal cleanup failed", Message: err.Error(), Buttons: []string{"OK"}})
		return true
	}
	return false
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	base := os.Getenv("MINI_ORCA_URL")
	if base == "" {
		base = "http://127.0.0.1:9090"
	}
	client, err := bridge.NewClient(base)
	if err != nil {
		return err
	}
	files, err := assets.Files()
	if err != nil {
		return err
	}
	app := &Desktop{client: client}
	return wails.Run(&options.App{
		Title: "Mini-Orca", Width: 1440, Height: 920, MinWidth: 900, MinHeight: 640,
		BackgroundColour: options.NewRGB(19, 22, 25),
		AssetServer:      &assetserver.Options{Assets: files},
		OnStartup:        func(ctx context.Context) { app.ctx = ctx },
		OnBeforeClose:    app.beforeClose, Bind: []interface{}{app},
		Mac: &mac.Options{TitleBar: mac.TitleBarDefault()},
	})
}
