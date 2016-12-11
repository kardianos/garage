package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"

	"github.com/kardianos/garage/comm"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func main() {
	sig := runOutput()

	on := fmt.Sprintf(":%d", comm.Port())

	cert, err := tls.X509KeyPair(comm.Cert(), comm.Key())
	if err != nil {
		log.Fatal("failed to load cert and key", err)
	}

	creds := credentials.NewServerTLSFromCert(&cert)
	s := grpc.NewServer(
		grpc.Creds(creds),
	)

	svr := &server{sig: sig}
	comm.RegisterGarageServer(s, svr)

	listener, err := net.Listen("tcp", on)
	if err != nil {
		log.Fatal("failed to listen", err)
	}

	err = s.Serve(listener)
	if err != nil {
		log.Fatal("failed to serve", err)
	}
}

type server struct {
	sig chan struct{}
}

func (s *server) Ping(ctx context.Context, noop *comm.Noop) (*comm.Noop, error) {
	return noop, nil
}
func (s *server) Toggle(ctx context.Context, noop *comm.Noop) (*comm.Noop, error) {
	s.sig <- struct{}{}
	return noop, nil
}

func runOutput() chan struct{} {
	sig := make(chan struct{}, 3)
	go outputLoop(sig)
	return sig
}
