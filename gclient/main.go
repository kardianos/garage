// Copyright 2015 Daniel Theophanes.
// Use of this source code is governed by a zlib-style
// license that can be found in the LICENSE file.package service

// Open the door
package main

import (
	"log"
	"net/http"

	"golang.org/x/mobile/app"
	"golang.org/x/mobile/event"
	"golang.org/x/mobile/gl"
)

func main() {
	vs := &viewState{}
	
	app.Main(func(a app.App) {
		var cfg event.Config
		for ev := range a.Events() {
			switch ev := event.Filter(ev).(type) {
			case event.Lifecycle:
			case event.Draw:
				vs.draw(cfg)
				a.EndDraw()
			case event.Change:
			case event.Config:
				cfg = ev
			case event.Touch:
				vs.touch(ev, cfg)
			}
		}
	})
}

func checkNetwork() {
	res, err := http.Get("http://golang.org/")
	if err != nil {
		log.Print(err)
		return
	}
	defer res.Body.Close()
}

var (
	tc = make(chan event.Change, 100)
)

type viewState struct {
	touchChange event.Change
	
	// Describe sequential regions.
}

func (vs *viewState) touch(t event.Touch, c event.Config) {
	log.Printf("Location: %v, Change: %v", t.Loc, t.Change)
	tc <- t.Change
}

func (vs *viewState) draw(c event.Config) {
	select {
	case change := <-tc:
		vs.touchChange = change
		gl.ClearColor(0, 1, 0, 1)
	default:
		gl.ClearColor(0, 0, 1, 1)
	}
	switch vs.touchChange {
	default:
		gl.ClearColor(0, 0, 1, 1)
	case event.ChangeNone:
		gl.ClearColor(0, 1, 1, 1)
	case event.ChangeOn:
		gl.ClearColor(1, 0, 1, 1)
	case event.ChangeOff:
		gl.ClearColor(0, 1, 0, 1)
	}
	gl.Clear(gl.COLOR_BUFFER_BIT)
}
