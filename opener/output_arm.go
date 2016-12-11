package main

import (
	"log"
	"time"

	"github.com/davecheney/gpio"
	"github.com/davecheney/gpio/rpi"
)

func outputLoop(sig chan struct{}) {
	pin, err := rpi.OpenPin(rpi.GPIO17, gpio.ModeOutput)
	if err != nil {
		log.Fatalf("Can't open pin: %s", err.Error())
	}
	defer pin.Close()
	pin.Set()
	for {
		select {
		case <-sig:
			pin.Clear()
			time.Sleep(time.Millisecond * 300)
			pin.Set()
			time.Sleep(time.Millisecond * 300)
		}
	}
}
