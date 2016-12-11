package comm

import (
	"context"
	"encoding/base64"
	"io"
	"log"
	"time"

	"github.com/dchest/spipe"
	"github.com/pkg/errors"
)

type CmdType byte

const (
	CmdRespOK    CmdType = 200
	CmdRespFail          = 500
	CmdReqPing           = 100
	CmdReqClose          = 101
	CmdReqToggle         = 150
)

func Port() int {
	return port
}

func Host() string {
	return host
}

func Key() []byte {
	v, err := base64.StdEncoding.DecodeString(keyValue)
	if err != nil {
		panic(err)
	}
	return v
}

type netError interface {
	Error() string
	Temporary() bool
	Timeout() bool
}

var ErrTimeout = errors.New("timeout")

func WriteCmd(timeout time.Duration, conn *spipe.Conn, cmd CmdType) error {
	//	err := conn.SetWriteDeadline(time.Now().Add(timeout))
	//	if err != nil {
	//		return errors.Wrap(err, "set write deadline")
	//	}
	// buf := []byte{byte(cmd)}
	buf := make([]byte, 100)
	buf[0] = byte(cmd)
	n, err := conn.Write(buf)
	if err == io.EOF {
		return err
	}
	if err != nil {
		return errors.Wrap(err, "cmd write")
	}
	if n != 100 {
		return errors.Errorf("failed to write 1 byte, wrote %d", n)
	}
	err = conn.Flush()
	if err == io.EOF {
		return err
	}
	if err != nil {
		return errors.Wrap(err, "cmd flush")
	}
	return nil
}

func ReadCmd(timeout time.Duration, conn *spipe.Conn) (CmdType, error) {
	//	err := conn.SetReadDeadline(time.Now().Add(timeout))
	//	if err != nil {
	//		return 0, errors.Wrap(err, "set read deadline")
	//	}
	buf := make([]byte, 100)
	n, err := conn.Read(buf)
	if err == io.EOF {
		return 0, err
	}
	if ierr, ok := err.(netError); ok {
		log.Println("Net Error")
		if ierr.Timeout() {
			log.Println("timeout")
			return 0, ErrTimeout
		}
	}

	if err != nil {
		return 0, errors.Wrap(err, "cmd read")
	}
	if n != 100 {
		return 0, errors.Errorf("failed to read 1 byte, read %d", n)
	}

	return CmdType(buf[0]), nil
}
func Handshake(timeout time.Duration, conn *spipe.Conn) error {
	err := conn.SetDeadline(time.Now().Add(timeout))
	done := make(chan struct{})
	go func() {
		err = conn.Handshake()
		done <- struct{}{}
	}()

	timer := time.NewTimer(timeout)
	select {
	case <-done:
		timer.Stop()
		return err
	case <-timer.C:
		conn.Close()
		return context.DeadlineExceeded
	}
}
