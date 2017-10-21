// +build !appengine

package log

import (
	"io"
	"log"
	"os"
)

func output() io.Writer {
	devnull, err := os.Open(os.DevNull)
	if err != nil {
		log.Fatal(err)
	}
	return devnull
}
