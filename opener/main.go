package main

import (
	"fmt"
	"io"
	"log"
	"time"

	"github.com/kardianos/garage/comm"

	"github.com/dchest/spipe"
)

func main() {
	sig := runOutput()

	on := fmt.Sprintf(":%d", comm.Port())
	listener, err := spipe.Listen([]byte("ABC"), "tcp", on)
	if err != nil {
		log.Fatalf("Failed to listen on port %q", on)
	}

	for {
		conn, err := listener.Accept()
		if err == io.EOF {
			log.Fatal("Listener closed")
		}
		if err != nil {
			log.Println("listener %v", err)
			continue
		}
		go func() {
			err := handle(sig, conn.(*spipe.Conn))
			if err == io.EOF {
				return
			}
			if err != nil {
				log.Printf("handle %+v", err)
			}
		}()
	}
}

func handle(sig chan struct{}, conn *spipe.Conn) error {
	opTimeout := time.Second * 3
	for {
		cmd, err := comm.ReadCmd(opTimeout, conn)
		if err == comm.ErrTimeout {
			continue
		}
		if err != nil {
			return err
		}
		switch cmd {
		case comm.CmdReqPing:
			err = comm.WriteCmd(opTimeout, conn, comm.CmdRespOK)
			if err != nil {
				return err
			}
		case comm.CmdReqClose:
			comm.WriteCmd(opTimeout, conn, comm.CmdRespOK)
			conn.Close()
			return nil
		case comm.CmdReqToggle:
			err = comm.WriteCmd(opTimeout, conn, comm.CmdRespOK)
			if err != nil {
				return err
			}
			sig <- struct{}{}
		}
	}
}

func runOutput() chan struct{} {
	sig := make(chan struct{}, 3)
	go outputLoop(sig)
	return sig
}
