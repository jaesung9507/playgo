package main

import (
	"context"
	"errors"
	"path/filepath"
	rt "runtime"
	"sync"

	"github.com/jaesung9507/playgo/stream"

	"github.com/deepch/vdk/format/mp4f"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx          context.Context
	streamClient stream.Client
	mp4Muxer     *mp4f.Muxer
	streamCtx    context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

func (a *App) SetAlwaysOnTop(b bool) {
	runtime.WindowSetAlwaysOnTop(a.ctx, b)
}

func (a *App) OpenFile() string {
	filePath, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Open File",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "Videos (*.flv;*.mp4;*.ts)",
				Pattern:     "*.flv;*.mp4;*.ts",
			},
		},
	})
	if err != nil {
		a.MsgBox(err.Error())
		return ""
	}

	if len(filePath) > 0 {
		switch rt.GOOS {
		case "windows":
			filePath = "file:///" + filepath.ToSlash(filePath)
		default:
			filePath = "file://" + filePath
		}
	}

	return filePath
}

func (a *App) Quit() {
	runtime.Quit(a.ctx)
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	runtime.EventsOn(a.ctx, "OnUpdateEnd", func(optionalData ...any) {
		a.wg.Add(1)
		a.streamLoop()
	})
}

func (a *App) MsgBox(msg string) {
	_, _ = runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
		Title:   "PlayGo",
		Message: msg,
		Buttons: []string{"OK"},
	})
}

func (a *App) CloseStream() {
	if a.cancel != nil {
		a.cancel()
		a.cancel = nil
	}
	a.wg.Wait()

	if a.streamClient != nil {
		a.streamClient.Close()
		a.streamClient = nil
	}

	if a.mp4Muxer != nil {
		a.mp4Muxer = nil
	}
}

func (a *App) initStream(client stream.Client, muxer *mp4f.Muxer) {
	a.streamClient = client
	a.mp4Muxer = muxer
}

func (a *App) streamLoop() {
	defer a.wg.Done()
	if a.streamClient != nil && a.mp4Muxer != nil {
		defer runtime.EventsEmit(a.ctx, "OnStreamStop")
		for {
			select {
			case <-a.streamCtx.Done():
				return
			case <-a.streamClient.CloseCh():
				return
			case packetAV := <-a.streamClient.PacketQueue():
				switch rt.GOOS {
				case "linux":
					packetAV.CompositionTime = 0
				}

				ready, buf, _ := a.mp4Muxer.WritePacket(*packetAV, false)
				if ready {
					runtime.EventsEmit(a.ctx, "OnFrame", buf)
				}
			}
		}
	}
}

func (a *App) PlayStream(url string) bool {
	a.streamCtx, a.cancel = context.WithCancel(a.ctx)

	client, err := stream.Dial(a.streamCtx, url)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			a.MsgBox(err.Error())
		}
		return false
	}

	codecData, err := stream.CodecData(a.streamCtx, client)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			a.MsgBox(err.Error())
		}
		return false
	}

	muxer := mp4f.NewMuxer(nil)
	if err = muxer.WriteHeader(codecData); err != nil {
		client.Close()
		a.MsgBox(err.Error())
		return false
	}
	meta, init := muxer.GetInit(codecData)
	a.initStream(client, muxer)
	runtime.EventsEmit(a.ctx, "OnInit", meta, init)

	return true
}
