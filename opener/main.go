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
	ossigs := make(chan os.Signal)
	signal.Notify(ossigs, os.Interrupt, os.Kill)
	go func() {
		<-ossigs
		quit()
		<-time.After(3 * time.Second)
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

	for {
		select {
		default:
		case <-ctx.Done():
			return
		}
		err = s.Serve(ctx)
		if err != nil {
			log.Println("failed to serve", err)
			time.Sleep(1 * time.Second)
			continue
		}
	}
}

type server struct {
	gc  comm.GarageClient
	sig chan struct{}
}

func (s *server) Serve(sctx context.Context) (err error) {
	ggc, err := s.gc.Garage(sctx)
	if err != nil {
		log.Println("Garage", err)
		return
	}
	err = s.runGarageService(ggc)
	if err != nil {
		log.Println("Garage Service", err)
		return
	}
	return
}

func (s *server) runGarageService(ggc comm.Garage_GarageClient) error {
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		defer ggc.CloseSend()

		for {
			select {
			case <-ggc.Context().Done():
				return
			case now := <-ticker.C:
				ggc.Send(&comm.FromGarage{TimeUnix: now.Unix()})
			}
		}
	}()
	for {
		select {
		default:
		case <-ggc.Context().Done():
			return nil
		}

		recv, err := ggc.Recv()
		if err != nil {
			log.Println("Recv", err)
			return err
		}
		if recv.Toggle {
			s.sig <- struct{}{}
		}
	}
	return nil
}

func runOutput() chan struct{} {
	sig := make(chan struct{}, 3)
	go outputLoop(sig)
	return sig
}
