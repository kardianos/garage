package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/kardianos/garage/comm"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func main() {
	sig := runOutput()

	ctx, quit := context.WithCancel(context.Background())
	ossigs := make(chan os.Signal, 3)
	signal.Notify(ossigs, os.Kill)
	go func() {
		<-ossigs
		quit()
		<-time.After(10 * time.Second)
		os.Exit(0)
	}()

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
		log.Fatal("dial", err)
		return
	}
	defer conn.Close()

	gc := comm.NewGarageClient(conn)

	s := &server{
		sig: sig,
		gc:  gc,
	}

	err = s.Serve(ctx)
	if err != nil {
		log.Fatal("failed to serve", err)
	}
}

type server struct {
	gc  comm.GarageClient
	sig chan struct{}
}

func (s *server) Serve(sctx context.Context) (err error) {
	for {
		select {
		default:
		case <-sctx.Done():
			return nil
		}

		ggc, err := s.gc.Garage(sctx, &comm.Noop{})
		if err != nil {
			log.Println("Garage", err)
			time.Sleep(1 * time.Second)
			continue
		}
		_, err = ggc.Recv()
		if err != nil {
			log.Println("Recv", err)
			time.Sleep(1 * time.Second)
			continue
		}
		s.sig <- struct{}{}

	}
	return nil
}

func runOutput() chan struct{} {
	sig := make(chan struct{}, 3)
	go outputLoop(sig)
	return sig
}
