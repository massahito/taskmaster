package server

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	t "github.com/massahito/taskmaster"
)

// Run executes RPC server for taskmaster.
//
// It take the configuration file path to its arguments,
// and block until signal is sent to this process.
//
// To stop gracefully, send SIGTERM/SIGINT.
//
// To reload configuration and restart server itself,
// send SIGHUP.
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
		err = t.SetLogger(cfg.Log)
		if err != nil {
			return err
		}

		// create controller
		ctrl := t.NewController(cfg)

		// start controller
		err = ctrl.Start()
		if err != nil {
			return err
		}

		// create TaskCmd
		tCmd := t.NewTaskCmd(cfg, ctrl)

		// cerate rpc Server
		server, err := newServer(cfg.Socket, tCmd)
		if err != nil {
			return err
		}

		go server.serve()

		select {
		case sig := <-sigCh:
			switch sig {
			case syscall.SIGTERM, syscall.SIGINT:
				// stopping receiving request and handle current request
				err = server.shutdown()
				if err != nil {
					slog.Error("server.shutdown", "error", err.Error())
				}
				// cleanup controller: stop all processes and goroutines
				err = ctrl.Shutdown()
				if err != nil {
					slog.Error("controller.Shutdown", "error", err.Error())
				}
				return nil
			case syscall.SIGHUP:
				// stopping receiving request and handle current request
				err = server.shutdown()
				if err != nil {
					slog.Error("server.shutdown", "error", err.Error())
				}

				// cleanup controller: stop all processes and goroutines
				err2 := ctrl.Shutdown()
				if err2 != nil {
					slog.Error("controller.Shutdown", "error", err.Error())
					return err2
				}

				// return err after finishing controller.Shutdown()
				if err != nil {
					return err
				}

				slog.Info("server reloaded")
			}
		}
	}

	return nil
}
