package server

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"net/rpc"
	"time"

	t "github.com/massahito/taskmaster"
)

type server struct {
	t      *t.TaskCmd
	path   string
	s      *http.Server
	cancel context.CancelFunc
}

func NewServer(path string, taskCmd *t.TaskCmd) *server {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)

	return &server{
		path: path,
		t:    taskCmd,
		s: &http.Server{
			BaseContext: func(l net.Listener) context.Context {
				return ctx
			},
		},
		cancel: cancel,
	}
}

func (s *server) Serve() error {

	l, err := net.Listen("unix", s.path)
	if err != nil {
		slog.Error("listen error", "error", err.Error())
		return err
	}

	rpc.DefaultServer = rpc.NewServer()
	rpc.DefaultServer.Register(s.t)
	rpc.DefaultServer.HandleHTTP(rpc.DefaultRPCPath, rpc.DefaultDebugPath)

	return s.s.Serve(l)
}

func (s *server) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := s.s.Shutdown(ctx)
	s.cancel()
	return err
}
