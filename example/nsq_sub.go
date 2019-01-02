// +build nsq_sub

package main

import (
	"github.com/mnu5/svckit/log"
	"github.com/mnu5/svckit/nsq"
	"github.com/mnu5/svckit/signal"
)

func main() {
	log.Debug("starting")

	handler := func(m *nsq.Message) error {
		log.S("msg", string(m.Body)).Info("sub")
		return nil
	}
	sub := nsq.Sub("nsq_example", handler)

	//clean exit
	signal.WaitForInterupt()
	log.Debug("stopping")
	sub.Close()
	log.Debug("stopped")
}
