package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/kardianos/garage/comm"
)

func main() {
	sig := runOutput()
	httpServer := &server{
		sig: sig,
	}

	on := fmt.Sprintf(":%d", comm.Port())

	cert, err := tls.X509KeyPair(comm.Cert(), comm.Key())
	if err != nil {
		log.Fatal("failed to load cert and key", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
		NextProtos:   []string{"h2", "http/1.1"},
		CipherSuites: comm.Ciphers,
	}

	svr := &http.Server{
		Addr:      on,
		Handler:   httpServer,
		TLSConfig: tlsConfig,

		ReadTimeout:       3000 * time.Millisecond,
		ReadHeaderTimeout: 3000 * time.Millisecond,
		WriteTimeout:      3000 * time.Millisecond,
	}

	listener, err := net.Listen("tcp", on)
	if err != nil {
		log.Fatal("failed to listen", err)
	}

	tlsListener := tls.NewListener(tcpKeepAliveListener{listener.(*net.TCPListener)}, tlsConfig)

	err = svr.Serve(tlsListener)
	if err != nil {
		log.Fatal("failed to serve", err)
	}
}

type tcpKeepAliveListener struct {
	*net.TCPListener
}

func (ln tcpKeepAliveListener) Accept() (c net.Conn, err error) {
	tc, err := ln.AcceptTCP()
	if err != nil {
		return
	}
	tc.SetKeepAlive(true)
	tc.SetKeepAlivePeriod(3 * time.Minute)
	return tc, nil
}

type server struct {
	sig chan struct{}
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get(comm.AuthHeader) != comm.AuthKey() {
		http.Error(w, "bad auth", http.StatusForbidden)
		return
	}
	switch r.URL.Path {
	default:
		http.Error(w, "not found", http.StatusNotFound)
	case comm.PathPing:
		s.Ping(w, r)
	case comm.PathToggle:
		s.Toggle(w, r)
	}
}

func (s *server) Ping(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("OK"))
}
func (s *server) Toggle(w http.ResponseWriter, r *http.Request) {
	s.sig <- struct{}{}
	w.Write([]byte("OK"))
}

func runOutput() chan struct{} {
	sig := make(chan struct{}, 3)
	go outputLoop(sig)
	return sig
}
