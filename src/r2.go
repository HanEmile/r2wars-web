package main

import (
	"log"

	"github.com/radareorg/r2pipe-go"
)

func r2cmd(r2p *r2pipe.Pipe, input string) (string, error) {
	log.Printf("> %s\n", input)

	// send a command
	buf1, err := r2p.Cmd(input)
	if err != nil {
		log.Println(err)
		return "", err
	}

	// return the result of the command as a string
	return buf1, nil
}
