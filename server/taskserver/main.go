package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	t "github.com/massahito/taskmaster"
	s "github.com/massahito/taskmaster/server"
	"github.com/sevlyar/go-daemon"
)

func main() {
	var pFlag = flag.Bool("p", false, "output samle configuration file to standard output")
	var cFlag = flag.String("c", "./taskmaster.yaml", "specify configuration files.")
	var nFlag = flag.Bool("n", false, "run taskserver in the foreground.")
	flag.Parse()

	if *pFlag {
		t.PrintExamle()
		return
	}

	// configuration check
	cfg, err := t.Parse(*cFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		os.Exit(1)
	}

	// socket path check
	if err := checkFileExist(cfg.Socket.Path); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		os.Exit(1)
	}

	// daemonize
	if !*nFlag {
		cntxt := &daemon.Context{}
		d, err := cntxt.Reborn()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err.Error())
			os.Exit(1)
		}
		if d != nil {
			return
		}
		defer cntxt.Release()
	}

	// create server
	err = s.Run(*cFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		os.Exit(1)
	}
}

func checkFileExist(path string) error {
	var err error
	if _, err = os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil
	}

	return fmt.Errorf("Error: Another program is already listening on a port that one of our HTTP servers is configured or the file already exists.")
}
