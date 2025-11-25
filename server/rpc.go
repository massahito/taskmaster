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
	t    *t.TaskCmd
	path string
	s    *http.Server
}

func NewServer(path string, taskCmd *t.TaskCmd) *server {
	return &server{
		path: path,
		t:    taskCmd,
		s:    &http.Server{},
	}
}

func (s *server) Serve() error {

	l, err := net.Listen("unix", s.path)
	if err != nil {
		slog.Error("listen error", "error", err.Error())
		return err
	}

	rpc.Register(s.t)
	rpc.HandleHTTP()

	return s.s.Serve(l)
}

func (s *server) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := s.s.Shutdown(ctx)
	return err
}
