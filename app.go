package main

import (
	"context"
	"rtsp-playgo/stream"
	rt "runtime"

	"github.com/deepch/vdk/format/mp4f"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx          context.Context
	streamClient stream.Client
	mp4Muxer     *mp4f.Muxer
	close        chan bool
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{close: make(chan bool)}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	runtime.EventsOn(a.ctx, "OnUpdateEnd", func(optionalData ...interface{}) {
		a.streamLoop()
	})
}

func (a *App) MsgBox(msg string) {
	_, _ = runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
		Title:   "RTSP PlayGo",
		Message: msg,
		Buttons: []string{"OK"},
	})
}

func (a *App) CloseStream() {
	if a.streamClient != nil {
		a.close <- true
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
	if a.streamClient != nil && a.mp4Muxer != nil {
		defer runtime.EventsEmit(a.ctx, "OnStreamStop")
		for {
			select {
			case <-a.close:
				return
			case <-a.streamClient.CloseCh():
				return
			case packetAV := <-a.streamClient.PacketQueue():
				switch rt.GOOS {
				case "darwin":
				default:
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
	client, err := stream.Dial(url)
	if err != nil {
		a.MsgBox(err.Error())
		return false
	}

	codecData, err := client.CodecData()
	if err != nil {
		client.Close()
		a.MsgBox(err.Error())
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
