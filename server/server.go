package server

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	t "github.com/massahito/taskmaster"
)

func Run(path string) error {
	// main signal handler
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	defer signal.Stop(sigCh)

	for {
		// parse config
		cfg, err := t.Parse(path)
		if err != nil {
			return err
		}
		// set logging
		t.SetLogger(cfg.Logging)

		// create controller
		ctrl := t.NewController(cfg)

		// start controller
		ctrl.Start()

		// create TaskCmd
		tCmd := t.NewTaskCmd(ctrl)

		// cerate rpc Server
		server := NewServer(cfg.Socket.Path, tCmd)
		go server.Serve()

		select {
		case sig := <-sigCh:
			switch sig {
			case syscall.SIGTERM, syscall.SIGINT:
				// stopping receiving request and handle current request
				server.Shutdown()
				// cleanup controller: stop all processes and goroutines
				err = tCmd.Shutdown(&t.CmdArg{}, &[]t.Proc{})
				return nil
			case syscall.SIGHUP:
				// stopping receiving request and handle current request
				server.Shutdown()
				// cleanup controller: stop all processes and goroutines
				err = tCmd.Shutdown(&t.CmdArg{}, &[]t.Proc{})
				slog.Info("server reloaded")
			}
		}
	}

	return nil
}
