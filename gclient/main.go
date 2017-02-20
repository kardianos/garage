package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net/http"
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
	client  *http.Client

	mu      sync.RWMutex
	pingOk  bool
	connErr error
}

func newViewState() *viewState {
	certpool := x509.NewCertPool()
	if !certpool.AppendCertsFromPEM(comm.CA()) { // Add CA public cert.
		panic("failed to add cert")
	}
	tlsConfig := &tls.Config{
		RootCAs:      certpool,
		CipherSuites: comm.Ciphers,
	}
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig:     tlsConfig,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}

	return &viewState{
		tc:          make(chan touch.Event, 100),
		lastDraw:    time.Now(),
		touchChange: touch.Event{Type: 200},
		client:      client,
	}
}

func (vs *viewState) startConn() {
	dnsName := fmt.Sprintf("%s:%d", comm.Host(), comm.Port())
	var err error

	err = vs.do(dnsName + comm.PathPing)
	vs.setPing(err)

	ticker := time.NewTicker(2500 * time.Second)

	for {
		select {
		case <-vs.onClose:
			return
		case tm := <-vs.toggle:
			if tm.Add(time.Second * 4).Before(time.Now()) {
				continue
			}
			err = vs.do(dnsName + comm.PathToggle)
			vs.setPing(err)

		case <-ticker.C:
			err = vs.do(dnsName + comm.PathPing)
			vs.setPing(err)
		}
	}
}
func (vs *viewState) do(to string) error {
	req, err := http.NewRequest("POST", "https://"+to, nil)
	if err != nil {
		return err
	}
	req.Header.Set(comm.AuthHeader, comm.AuthKey())
	resp, err := vs.client.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("code %d", resp.StatusCode)
	}
	if resp.Body != nil {
		resp.Body.Close()
	}
	return nil
}
func (vs *viewState) setPing(err error) {
	vs.mu.Lock()
	vs.pingOk = err == nil
	vs.connErr = err
	vs.mu.Unlock()
	fmt.Println(err)
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
		msg = connErr.Error()
	}

	vs.sq.Draw(glctx, sz, "Garage door opener", msg)
}
