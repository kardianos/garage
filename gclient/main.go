package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"log"
	"net"
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

	mu      sync.RWMutex
	pingOk  bool
	connErr error

	tlsConfig *tls.Config
	cancel    func()
}

func newViewState() *viewState {
	certpool := x509.NewCertPool()
	if !certpool.AppendCertsFromPEM(comm.CA()) { // Add CA public cert.
		panic("failed to add cert")
	}
	tlsConfig := &tls.Config{
		RootCAs:      certpool,
		CipherSuites: comm.Ciphers,
		ServerName:   comm.Host(),
	}

	return &viewState{
		tc:          make(chan touch.Event, 100),
		lastDraw:    time.Now(),
		touchChange: touch.Event{Type: 200},

		tlsConfig: tlsConfig,
	}
}

type dialer struct {
	tlsConfig *tls.Config
	host      string
	port      int

	conn    *tls.Conn
	err     error
	encoder *json.Encoder
	decoder *json.Decoder
}

func (d *dialer) open(ctx context.Context) error {
	if d.conn != nil {
		d.conn.Close()
	}
	dnsCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	names, err := net.DefaultResolver.LookupIPAddr(dnsCtx, d.host)
	cancel()
	if err != nil {
		return err
	}
	if len(names) == 0 {
		return fmt.Errorf("no addrs for host %q", d.host)
	}
	result := make(chan *tls.Conn, 2)
	connCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	for _, n := range names {
		go func(to string) {
			dialer := net.Dialer{
				KeepAlive: 15 * time.Second,
			}
			conn, err := dialer.DialContext(connCtx, "tcp", to)
			if err != nil {
				return
			}
			result <- tls.Client(conn, d.tlsConfig)
		}(fmt.Sprintf("%s:%d", n.String(), d.port))
	}
	select {
	case <-connCtx.Done():
		return connCtx.Err()
	case r := <-result:
		d.conn = r
		d.encoder = json.NewEncoder(d.conn)
		d.decoder = json.NewDecoder(d.conn)
	}
	return nil
}
func (d *dialer) send(ctx context.Context, req comm.Request) (comm.Response, error) {
	resp := comm.Response{}
	coderDone := make(chan error)
	go func() {
		coderDone <- d.encoder.Encode(req)
	}()
	select {
	case <-ctx.Done():
		go func() {
			<-coderDone
		}()
		return resp, ctx.Err()
	case err := <-coderDone:
		if err != nil {
			return resp, err
		}
	}

	go func() {
		coderDone <- d.decoder.Decode(&resp)
	}()
	select {
	case <-ctx.Done():
		go func() {
			<-coderDone
		}()
		return resp, ctx.Err()
	case err := <-coderDone:
		if err != nil {
			return resp, err
		}
	}
	return resp, nil
}

func (vs *viewState) startConn(ctx context.Context) {
	d := &dialer{host: comm.Host(), port: comm.Port(), tlsConfig: vs.tlsConfig}
	var err error
	for {
		err = d.open(ctx)
		vs.setPing(err)
		if err == nil {
			break
		}
	}
	if d.conn != nil {
		defer d.conn.Close()
	}
	go func() {
		<-vs.onClose
		vs.cancel()
	}()

	ticker := time.NewTicker(2500 * time.Millisecond)
	for {
		select {
		case tm := <-vs.toggle:
			if tm.Add(time.Second * 20).Before(time.Now()) {
				continue
			}
			_, err := d.send(ctx, comm.Request{Auth: comm.AuthKey(), Type: comm.RequestToggle})
			vs.setPing(err)

		case <-ticker.C:
			_, err := d.send(ctx, comm.Request{Auth: comm.AuthKey(), Type: comm.RequestPing})
			vs.setPing(err)
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
	var ctx context.Context
	ctx, vs.cancel = context.WithCancel(context.Background())
	go vs.startConn(ctx)
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
