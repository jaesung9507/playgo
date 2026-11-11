package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	rt "runtime"
	"sync"

	"github.com/jaesung9507/playgo/stream"
	"github.com/jaesung9507/playgo/stream/format/fmp4"
	"github.com/jaesung9507/playgo/stream/route"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type PlayerService struct {
	app          *application.App
	ctx          context.Context
	streamClient stream.Client
	mp4Muxer     *fmp4.Muxer
	streamCtx    context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
}

func NewPlayerService(app *application.App) *PlayerService {
	return &PlayerService{app: app}
}

func (s *PlayerService) Window() application.Window {
	w, _ := s.app.Window.Get("player")
	return w
}

func (s *PlayerService) SetAlwaysOnTop(b bool) {
	s.Window().SetAlwaysOnTop(b)
}

func (s *PlayerService) OpenFile() string {
	pattern := route.SupportedFilePatterns()
	filePath, err := s.app.Dialog.OpenFileWithOptions(&application.OpenFileDialogOptions{
		Title: "Open File",
		Filters: []application.FileFilter{
			{
				DisplayName: fmt.Sprintf("Videos (%s)", pattern),
				Pattern:     pattern,
			},
		},
	}).PromptForSingleSelection()
	if err != nil {
		s.MsgBox(err.Error())
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

func (s *PlayerService) Quit() {
	s.app.Quit()
}

func (s *PlayerService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	s.ctx = ctx

	if screens := s.app.Screen.GetAll(); len(screens) > 0 {
		primaryScreen := s.app.Screen.GetPrimary()
		if primaryScreen == nil {
			primaryScreen = screens[0]
		}

		var width, height int
		if primaryScreen.Size.Width >= primaryScreen.Size.Height {
			width = min(int(float64(primaryScreen.Size.Width)*0.5), 1024)
		} else {
			width = min(int(float64(primaryScreen.Size.Height)*0.5), 1024)
		}
		height = int(float64(width) * 0.75)

		s.Window().SetSize(width, height)
		s.Window().Center()
	}

	s.app.Event.On("OnUpdateEnd", func(event *application.CustomEvent) {
		s.wg.Add(1)
		s.streamLoop()
	})

	return nil
}

func (s *PlayerService) MsgBox(msg string) {
	s.app.Dialog.Info().
		SetTitle("PlayGo").
		SetMessage(msg).
		Show()
}

func (s *PlayerService) CloseStream() {
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	s.wg.Wait()

	if s.streamClient != nil {
		s.streamClient.Close()
		s.streamClient = nil
	}

	if s.mp4Muxer != nil {
		s.mp4Muxer = nil
	}
}

func (s *PlayerService) initStream(client stream.Client, muxer *fmp4.Muxer) {
	s.streamClient = client
	s.mp4Muxer = muxer
}

func (s *PlayerService) streamLoop() {
	defer s.wg.Done()
	if s.streamClient != nil && s.mp4Muxer != nil {
		defer s.app.Event.Emit("OnStreamStop")
		for {
			select {
			case <-s.streamCtx.Done():
				return
			case <-s.streamClient.CloseCh():
				return
			case packet := <-s.streamClient.PacketQueue():
				switch rt.GOOS {
				case "linux":
					packet.CompositionTime = 0
				}

				if buf, _ := s.mp4Muxer.WritePacket(*packet); buf != nil {
					s.app.Event.Emit("OnFrame", buf)
				}
			}
		}
	}
}

func (s *PlayerService) PlayStream(url string) (result bool) {
	s.streamCtx, s.cancel = context.WithCancel(s.ctx)
	c, err := route.NewClient(url)
	if err != nil {
		s.MsgBox(err.Error())
		return false
	}

	defer func() {
		if !result {
			c.Close()
		}
	}()

	if err = stream.Dial(s.streamCtx, c); err != nil {
		if !errors.Is(err, context.Canceled) {
			s.MsgBox(err.Error())
		}
		return false
	}

	codecData, err := stream.CodecData(s.streamCtx, c)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			s.MsgBox(err.Error())
		}
		return false
	}

	if len(codecData) <= 0 {
		s.MsgBox("not found codec info")
		return false
	}

	muxer := fmp4.NewMuxer()
	meta, init, err := muxer.WriteHeader(codecData)
	if err != nil {
		s.MsgBox(err.Error())
		return false
	}

	secured, trusted, secureInfo := c.Secure()
	s.app.Event.Emit("OnSecureInfo", secured, trusted, secureInfo)

	s.initStream(c, muxer)
	s.app.Event.Emit("OnInit", meta, init)

	return true
}
