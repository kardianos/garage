package main

import (
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"github.com/kardianos/garage/comm"

	"github.com/kardianos/service"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var _ service.Interface = &program{}

type program struct {
	quit func()
}

func (p *program) Start(svc service.Service) error {
	ctx, quit := context.WithCancel(context.Background())
	p.quit = quit

	on := fmt.Sprintf(":%d", comm.Port())

	cert, err := tls.X509KeyPair(comm.Cert(), comm.Key())
	if err != nil {
		return fmt.Errorf("failed to load cert and key", err)
	}
	creds := credentials.NewServerTLSFromCert(&cert)

	s := grpc.NewServer(
		grpc.Creds(creds),
	)
	listener, err := net.Listen("tcp", on)
	if err != nil {
		return fmt.Errorf("failed to listen on %q", on)
	}
	m := &mirror{
		appCtx: ctx,
		g:      make(map[comm.Garage_GarageServer]chan time.Time, 3),
	}
	comm.RegisterGarageServer(s, m)
	go func() {
		err = s.Serve(listener)
		if err != nil {
			log.Fatal("failed to serve", err)
		}
	}()

	return nil
}

func (p *program) Stop(s service.Service) error {
	// Stop should not block. Return with a few seconds.
	p.quit()
	go func() {
		<-time.After(3 * time.Second)
		os.Exit(0)
	}()
	return nil
}

var logger service.Logger

func main() {
	svcFlag := flag.String("service", "", "control the service")
	flag.Parse()

	svcConfig := &service.Config{
		Name:        "garagemirror",
		DisplayName: "Garage Mirror",
		Description: "Interface between the garage remote and device.",
	}

	prg := &program{}
	s, err := service.New(prg, svcConfig)
	if err != nil {
		log.Fatal(err)
	}
	logger, err = s.Logger(nil)
	if err != nil {
		log.Fatal(err)
	}
	if len(*svcFlag) != 0 {
		err := service.Control(s, *svcFlag)
		if err != nil {
			log.Printf("Valid actions: %q\n", service.ControlAction)
			log.Fatal(err)
		}
		return
	}
	err = s.Run()
	if err != nil {
		logger.Error(err)
	}
}

type mirror struct {
	appCtx context.Context
	sync.RWMutex
	g map[comm.Garage_GarageServer]chan time.Time
}

func (m *mirror) Ping(ctx context.Context, _ *comm.PingReq) (*comm.PingResp, error) {
	m.RLock()
	ct := len(m.g)
	m.RUnlock()

	if ct == 0 {
		return nil, errors.New("Garage Not Registered")
	}

	return &comm.PingResp{}, nil
}
func (m *mirror) Toggle(ctx context.Context, req *comm.ToggleReq) (*comm.ToggleResp, error) {
	sent := false
	m.RLock()
	for _, action := range m.g {
		sent = true
		action <- time.Unix(req.TimeUnix, 0)
	}
	m.RUnlock()

	if !sent {
		return nil, errors.New("Garage Not Registered")
	}

	return &comm.ToggleResp{}, nil
}

func (m *mirror) Garage(ggs comm.Garage_GarageServer) error {
	notify := make(chan time.Time, 6)
	m.Lock()
	m.g[ggs] = notify
	m.Unlock()

	defer func() {
		m.Lock()
		delete(m.g, ggs)
		m.Unlock()
	}()

	recv := make(chan *comm.FromGarage)
	go func() {
		for {
			fg, err := ggs.Recv()
			if err != nil {
				return
			}
			recv <- fg
		}
	}()

	ctx := ggs.Context()
	for {
		select {
		case <-m.appCtx.Done():
			return nil
		case <-ctx.Done():
			return nil
		case n := <-notify:
			err := ggs.Send(&comm.ToGarage{TimeUnix: n.Unix(), Toggle: true})
			if err != nil {
				return fmt.Errorf("garage send %v", err)
			}
		case fg := <-recv:
			err := ggs.Send(&comm.ToGarage{TimeUnix: fg.TimeUnix})
			if err != nil {
				return fmt.Errorf("garage send %v", err)
			}

		}

	}
	return nil
}
