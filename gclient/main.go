package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/kardianos/garage/comm"

	"github.com/cloudflare/backoff"
	"golang.org/x/mobile/app"
	"golang.org/x/mobile/event/lifecycle"
	"golang.org/x/mobile/event/paint"
	"golang.org/x/mobile/event/size"
	"golang.org/x/mobile/event/touch"
	"golang.org/x/mobile/gl"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
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

	toggle  chan time.Time
	onClose chan struct{}

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

// TODO Specify auth.
// TODO Create new certs.

func (vs *viewState) startConn() {
	certpool := x509.NewCertPool()
	if !certpool.AppendCertsFromPEM(comm.CA()) { // Add CA public cert.
		panic("failed to add cert")
	}
	creds := credentials.NewTLS(&tls.Config{
		RootCAs: certpool,
	})

	errClosed := errors.New("closed")
	backer := backoff.New(time.Millisecond*500, time.Millisecond*10)

	do := func() error {
		conn, err := grpc.Dial(fmt.Sprintf("%s:%d", comm.Host(), comm.Port()),
			grpc.WithTimeout(time.Second*3),
			grpc.FailOnNonTempDialError(true),
			grpc.WithTransportCredentials(creds),
			//			grpc.WithPerRPCCredentials(jwtCreds),
		)
		if err != nil {
			return err
		}
		defer conn.Close()

		client := comm.NewGarageClient(conn)
		ctx := context.TODO()
		noop := &comm.Noop{}
		_, err = client.Ping(ctx, noop)
		if err != nil {
			return err
		}
		vs.mu.Lock()
		vs.connErr = nil
		vs.pingOk = true
		vs.mu.Unlock()

		ticker := time.NewTicker(time.Millisecond * 3000)
		defer ticker.Stop()

		for {
			select {
			case <-vs.onClose:
				return errClosed
			case tm := <-vs.toggle:
				if tm.Add(time.Second * 4).Before(time.Now()) {
					continue
				}
				_, err = client.Toggle(ctx, noop)
				if err != nil {
					return err
				}
			case <-ticker.C:
				_, err = client.Ping(ctx, noop)
				if err != nil {
					return err
				}
				vs.mu.Lock()
				vs.pingOk = true
				vs.mu.Unlock()
			}
		}
	}
	for {
		err := do()
		if err == errClosed {
			return
		}
		if err != nil {
			vs.mu.Lock()
			vs.connErr = err
			vs.pingOk = false
			vs.mu.Unlock()

			time.Sleep(backer.Duration())
			continue
		}
		backer.Reset()
		continue
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

	vs.mu.Lock()
	vs.toggle = make(chan time.Time)
	vs.onClose = make(chan struct{})
	vs.pingOk = false
	vs.mu.Unlock()
	go vs.startConn()
}
func (vs *viewState) end(glctx gl.Context) {
	vs.sq.Close(glctx)
	close(vs.onClose)
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
	if vs.percentColor < .1 {
		vs.percentColor = .1
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
		msg = connErr.Error()
	}

	vs.sq.Draw(glctx, sz, "Garage door opener", msg)
}
