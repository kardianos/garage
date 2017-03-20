package main

import (
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/kardianos/garage/comm"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func main() {
	ctx, quit := context.WithCancel(context.Background())
	ossigs := make(chan os.Signal, 3)
	signal.Notify(ossigs, os.Kill)
	go func() {
		<-ossigs
		quit()
		<-time.After(10 * time.Second)
		os.Exit(0)
	}()

	runServer(ctx)
}

func runServer(ctx context.Context) {
	on := fmt.Sprintf(":%d", comm.Port())

	cert, err := tls.X509KeyPair(comm.Cert(), comm.Key())
	if err != nil {
		log.Fatal("failed to load cert and key", err)
	}
	creds := credentials.NewServerTLSFromCert(&cert)

	s := grpc.NewServer(
		grpc.Creds(creds),
	)
	listener, err := net.Listen("tcp", on)
	if err != nil {
		log.Fatalf("failed to listen on %q", on)
	}
	m := &mirror{
		g: make(map[comm.Garage_GarageServer]chan time.Time, 3),
	}
	comm.RegisterGarageServer(s, m)
	err = s.Serve(listener)
	if err != nil {
		log.Fatal("failed to serve", err)
	}
}

type mirror struct {
	sync.RWMutex
	g map[comm.Garage_GarageServer]chan time.Time
}

func (m *mirror) Ping(ctx context.Context, _ *comm.Noop) (*comm.Noop, error) {
	m.RLock()
	ct := len(m.g)
	m.RUnlock()

	if ct == 0 {
		return nil, errors.New("Garage Not Registered")
	}

	return &comm.Noop{}, nil
}
func (m *mirror) Toggle(ctx context.Context, _ *comm.Noop) (*comm.Noop, error) {
	sent := false
	m.RLock()
	now := time.Now()
	for _, action := range m.g {
		sent = true
		action <- now
	}
	m.RUnlock()

	if !sent {
		return nil, errors.New("Garage Not Registered")
	}

	return &comm.Noop{}, nil
}
func (m *mirror) Garage(_ *comm.Noop, ggs comm.Garage_GarageServer) error {
	notify := make(chan time.Time, 6)
	m.Lock()
	m.g[ggs] = notify
	m.Unlock()

	defer func() {
		m.Lock()
		delete(m.g, ggs)
		m.Unlock()
	}()

	ctx := ggs.Context()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-notify:
			err := ggs.Send(&comm.Noop{})
			if err != nil {
				return fmt.Errorf("garage send %v", err)
			}
		}

	}
	return nil
}
