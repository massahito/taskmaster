package main

import (
	"flag"
	"fmt"
	"os"

	s "github.com/massahito/taskmaster/server"
)

func main() {
	var cFlag = flag.String("c", "./taskmaster.yaml", "specify configuration files. Default: ./taskmaster.yaml")
	flag.Parse()

	err := s.Run(*cFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		os.Exit(1)
	}
}
