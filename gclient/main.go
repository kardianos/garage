// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build darwin linux

// An app that draws a green triangle on a red background.
//
// Note: This demo is an early preview of Go 1.5. In order to build this
// program as an Android APK using the gomobile tool.
//
// See http://godoc.org/golang.org/x/mobile/cmd/gomobile to install gomobile.
//
// Get the basic example and use gomobile to build or install it on your device.
//
//   $ go get -d golang.org/x/mobile/example/basic
//   $ gomobile build golang.org/x/mobile/example/basic # will build an APK
//
//   # plug your Android device to your computer or start an Android emulator.
//   # if you have adb installed on your machine, use gomobile install to
//   # build and deploy the APK to an Android target.
//   $ gomobile install golang.org/x/mobile/example/basic
//
// Switch to your device or emulator to start the Basic application from
// the launcher.
// You can also run the application on your desktop by running the command
// below. (Note: It currently doesn't work on Windows.)
//   $ go install golang.org/x/mobile/example/basic && basic
package main

import (
	"log"
	"os"

	"golang.org/x/mobile/app"
	"golang.org/x/mobile/event/lifecycle"
	"golang.org/x/mobile/event/paint"
	"golang.org/x/mobile/event/size"
	"golang.org/x/mobile/event/touch"
	"golang.org/x/mobile/exp/gl/glutil"
	"golang.org/x/mobile/gl"
)

var (
	images *glutil.Images

	green  float32
	touchX float32
	touchY float32
)

func main() {
	app.Main(func(a app.App) {
		var glctx gl.Context
		var sz size.Event

		vs := newViewState()
		for e := range a.Events() {
			switch e := a.Filter(e).(type) {
			case lifecycle.Event:
				switch e.Crosses(lifecycle.StageVisible) {
				case lifecycle.CrossOn:
					glctx, _ = e.DrawContext.(gl.Context)
					vs.start(glctx)
					a.Send(paint.Event{})
				case lifecycle.CrossOff:
					vs.end(glctx)
					glctx = nil
				}
			case size.Event:
				sz = e
				touchX = float32(sz.WidthPx / 2)
				touchY = float32(sz.HeightPx / 2)
			case paint.Event:
				if glctx == nil || e.External {
					// As we are actively painting as fast as
					// we can (usually 60 FPS), skip any paint
					// events sent by the system.
					continue
				}

				vs.draw(glctx, sz)
				a.Publish()
				// Drive the animation by preparing to paint the next frame
				// after this one is shown.
				a.Send(paint.Event{})
			case touch.Event:
				vs.touch(e)
				touchX = e.X
				touchY = e.Y
			}
		}
	})
}

type viewState struct {
	touchChange touch.Event

	tc chan touch.Event

	sq *Square
}

func newViewState() *viewState {
	return &viewState{
		tc: make(chan touch.Event, 100),
	}
}

func (vs *viewState) start(glctx gl.Context) {
	var err error
	vs.sq, err = NewSquare(glctx, 0.01, 1, 0.2)
	if err != nil {
		log.Printf("Failed to create square: %v", err)
		os.Exit(1)
	}
	vs.sq.SetLocation(0, 10)
}
func (vs *viewState) end(glctx gl.Context) {
	vs.sq.Close(glctx)
}
func (vs *viewState) touch(t touch.Event) {
	log.Printf("Location: %vx%v, Change: %v", t.X, t.Y, t.Type)
	vs.sq.SetLocation(t.X, t.Y)
	vs.tc <- t
}

func (vs *viewState) draw(glctx gl.Context, sz size.Event) {
	select {
	case change := <-vs.tc:
		vs.touchChange = change
		glctx.ClearColor(0, 1, 0, 1)
	default:
		glctx.ClearColor(0, 0, 1, 1)
	}
	switch vs.touchChange.Type {
	default:
		glctx.ClearColor(0, 0, 1, 1)
	case touch.TypeMove:
		glctx.ClearColor(0, 1, 1, 1)
	case touch.TypeBegin:
		glctx.ClearColor(1, 0, 1, 1)
	case touch.TypeEnd:
		glctx.ClearColor(0, 1, 0, 1)
	}

	glctx.Clear(gl.COLOR_BUFFER_BIT)

	vs.sq.Draw(glctx, sz, "Garage door opener", "Tap to toggle garage door")
}
