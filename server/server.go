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
			slog.Error("config.Parse", "error", err.Error())
			return err
		}

		// set logging
		err = t.SetLogger(cfg.Log)
		if err != nil {
			slog.Error("taskmaster.SetLogger", "error", err.Error())
			return err
		}

		// create controller
		ctrl := t.NewController(cfg)

		// start controller
		ctrl.Start()

		// create TaskCmd
		tCmd := t.NewTaskCmd(cfg, ctrl)

		// cerate rpc Server
		server, err := NewServer(cfg.Socket.Path, tCmd)
		if err != nil {
			slog.Error("NewServer", "error", err.Error())
			return err
		}

		go server.Serve()

		select {
		case sig := <-sigCh:
			switch sig {
			case syscall.SIGTERM, syscall.SIGINT:
				// stopping receiving request and handle current request
				err = server.Shutdown()
				if err != nil {
					slog.Error("server.Shutdown", "error", err.Error())
				}
				// cleanup controller: stop all processes and goroutines
				err = ctrl.Shutdown()
				if err != nil {
					slog.Error("controller.Shutdown", "error", err.Error())
				}
				return nil
			case syscall.SIGHUP:
				// stopping receiving request and handle current request
				err = server.Shutdown()
				if err != nil {
					slog.Error("server.Shutdown", "error", err.Error())
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
