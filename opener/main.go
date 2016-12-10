package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/davecheney/gpio"
	"github.com/davecheney/gpio/rpi"
)

func main() {
	sig := runOutput()
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Query().Get("k")
		if key != "ABCZYX" {
			return
		}
		fmt.Fprintf(w, "Door Toggle")
		sig <- struct{}{}
	})
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatalf("Failed to serve http: %s", err.Error())
	}
}

func runOutput() chan struct{} {
	sig := make(chan struct{}, 3)
	go outputLoop(sig)
	return sig
}

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
