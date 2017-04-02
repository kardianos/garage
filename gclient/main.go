package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/kardianos/garage/comm"

	"golang.org/x/mobile/app"
	"golang.org/x/mobile/event/lifecycle"
	"golang.org/x/mobile/event/paint"
	"golang.org/x/mobile/event/size"
	"golang.org/x/mobile/event/touch"
	"golang.org/x/mobile/gl"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func main() {
	app.Main(func(a app.App) {
		var glctx gl.Context
		var sz size.Event

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		vs := newViewState()
		vs.startConn(ctx)

		for e := range a.Events() {
			switch e := a.Filter(e).(type) {
			case lifecycle.Event:
				switch e.Crosses(lifecycle.StageAlive) {
				case lifecycle.CrossOn:
					log.Println("start")
				case lifecycle.CrossOff:
					log.Println("end")
					cancel()
				}
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
			}
		}
	})
}

type viewState struct {
	touchChange  touch.Event
	tc           chan touch.Event
	percentColor float32
	lastDraw     time.Time
	lastTouch    time.Time

	sq *Square

	toggle chan time.Time

	mu      sync.RWMutex
	pingOk  bool
	connErr error
}

func newViewState() *viewState {
	return &viewState{
		tc:          make(chan touch.Event, 100),
		lastDraw:    time.Now(),
		touchChange: touch.Event{Type: 200},
	}
}

func (vs *viewState) runConn(ctx context.Context) {
	certpool := x509.NewCertPool()
	if !certpool.AppendCertsFromPEM(comm.CA()) { // Add CA public cert.
		panic("failed to add cert")
	}
	creds := credentials.NewTLS(&tls.Config{
		RootCAs: certpool,
	})
	dnsName := fmt.Sprintf("%s:%d", comm.Host(), comm.Port())
	conn, err := grpc.DialContext(ctx, dnsName,
		grpc.WithTransportCredentials(creds),
	)
	if err != nil {
		vs.setPing(err)
		return
	}
	defer conn.Close()

	gc := comm.NewGarageClient(conn)
	_, err = gc.Ping(ctx, &comm.PingReq{})
	vs.setPing(err)

	for {
		ticker := time.NewTicker(2500 * time.Millisecond)
		for {
			select {
			case <-ctx.Done():
				return
			case tm := <-vs.toggle:
				if tm.Add(time.Second * 10).Before(time.Now()) {
					continue
				}
				_, err = gc.Toggle(ctx, &comm.ToggleReq{TimeUnix: tm.Unix()})
				vs.setPing(err)

			case tm := <-ticker.C:
				_, err = gc.Ping(ctx, &comm.PingReq{TimeUnix: tm.Unix()})
				vs.setPing(err)
			}
		}
	}
}
func (vs *viewState) setPing(err error) {
	vs.mu.Lock()
	vs.pingOk = err == nil
	vs.connErr = err
	vs.mu.Unlock()
	if err != nil {
		log.Println(err)
	}
}

func (vs *viewState) startConn(ctx context.Context) {
	vs.mu.Lock()
	vs.toggle = make(chan time.Time)
	vs.pingOk = false
	vs.connErr = nil
	vs.mu.Unlock()

	go vs.runConn(ctx)
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
	now := time.Now()
	diff := now.Sub(vs.lastDraw)
	vs.percentColor -= float32(diff.Seconds() * 0.5)
	if vs.percentColor < .25 {
		vs.percentColor = .25
	}
	vs.lastDraw = now

	select {
	case change := <-vs.tc:
		vs.touchChange = change
	default:
	}

	switch vs.touchChange.Type {
	case touch.TypeMove:
	case touch.TypeBegin:
		now := time.Now()
		if vs.lastTouch.Add(time.Millisecond * 1000).Before(now) {
			vs.lastTouch = now
			vs.percentColor = 0.5
			select {
			default:
			case vs.toggle <- now:
			}
		}
	case touch.TypeEnd:
	}

	var pingOk bool
	var connErr error
	vs.mu.RLock()
	pingOk = vs.pingOk
	connErr = vs.connErr
	vs.mu.RUnlock()

	var r, g, b = vs.percentColor, vs.percentColor, vs.percentColor
	if pingOk {
		r, b = 0, 0
	} else {
		g, b = 0, 0
	}

	glctx.ClearColor(r, g, b, 1)
	glctx.Clear(gl.COLOR_BUFFER_BIT)

	msg := "Tap to toggle garage door"
	if connErr != nil {
		msg = grpc.ErrorDesc(connErr)
	}

	vs.sq.Draw(glctx, sz, "Garage door opener", msg)
}
