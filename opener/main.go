package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"time"

	"github.com/kardianos/garage/comm"
)

func main() {
	sig := runOutput()

	on := fmt.Sprintf(":%d", comm.Port())

	cert, err := tls.X509KeyPair(comm.Cert(), comm.Key())
	if err != nil {
		log.Fatal("failed to load cert and key", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
		// NextProtos:   []string{"h2", "http/1.1"},
		CipherSuites: comm.Ciphers,
	}

	listener, err := net.Listen("tcp", on)
	if err != nil {
		log.Fatal("failed to listen", err)
	}

	tlsListener := tls.NewListener(listener, tlsConfig)
	s := &server{
		sig: sig,
		l:   tlsListener,
	}

	ctx, quit := context.WithCancel(context.Background())
	ossigs := make(chan os.Signal, 3)
	signal.Notify(ossigs, os.Kill)
	go func() {
		<-ossigs
		quit()
		<-time.After(10 * time.Second)
		os.Exit(0)
	}()
	err = s.Serve(ctx)
	if err != nil {
		log.Fatal("failed to serve", err)
	}
}

type server struct {
	l   net.Listener
	sig chan struct{}
}

func (s *server) Serve(sctx context.Context) (err error) {
	for {
		select {
		default:
		case <-sctx.Done():
			return nil
		}
		var conn net.Conn
		conn, err = s.l.Accept()
		ctx, cancel := context.WithTimeout(sctx, 3*time.Minute)
		err = s.conn(ctx, conn, err)
		cancel()
		if err != nil {
			log.Println("conn", err)
			continue
		}
	}
	return nil
}

func ewrap(err error, msg string) error {
	return fmt.Errorf("%s: %v", msg, err)
}

var (
	errBadAuth = errors.New("bad auth")
)

func (s *server) conn(ctx context.Context, conn net.Conn, err error) error {
	if err != nil {
		return ewrap(err, "accept")
	}
	defer conn.Close()

	decode := json.NewDecoder(conn)
	encode := json.NewEncoder(conn)
	for {
		select {
		default:
		case <-ctx.Done():
			return nil
		}
		req := comm.Request{}
		err = decode.Decode(&req)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return ewrap(err, "json decode")
		}
		if req.Auth != comm.AuthKey() {
			return errBadAuth
		}
		closeConn := false
		var resp comm.Response
		switch req.Type {
		default:
			return fmt.Errorf("unknown type %q", req.Type)
		case comm.RequestPing:
			resp = s.ping()
		case comm.RequestToggle:
			resp = s.toggle()
		case comm.RequestClose:
			closeConn = true
			resp.OK = true
		}

		err = encode.Encode(resp)
		if err != nil {
			return ewrap(err, "json encode")
		}
		if closeConn {
			return nil
		}
	}

	return nil
}

func (s *server) toggle() comm.Response {
	s.sig <- struct{}{}
	return comm.Response{OK: true}
}

func (s *server) ping() comm.Response {
	return comm.Response{OK: true}
}

func runOutput() chan struct{} {
	sig := make(chan struct{}, 3)
	go outputLoop(sig)
	return sig
}
