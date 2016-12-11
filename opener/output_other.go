// +build !arm

package main

import (
	"log"
)

func outputLoop(sig chan struct{}) {
	for {
		select {
		case <-sig:
			log.Println("toggle door")
		}
	}
}
