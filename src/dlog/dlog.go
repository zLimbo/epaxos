package dlog

import (
	"fmt"
	"log"
)

const DLOG = false

func Printf(format string, v ...interface{}) {
	if !DLOG {
		return
	}
	// log.Printf(format, v...)
	log.Output(2, fmt.Sprintf(format, v...))
}

func Println(v ...interface{}) {
	if !DLOG {
		return
	}
	log.Output(2, fmt.Sprintln(v...))
	// log.Println(v...)
}
