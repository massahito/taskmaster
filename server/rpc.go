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
	s    *http.Server
	l    net.Listener
	path string
}

func NewServer(path string, taskCmd *t.TaskCmd) (*server, error) {
	l, err := net.Listen("unix", path)
	if err != nil {
		slog.Error("NewServer:", "error", err.Error())
		return nil, err
	}

	http.DefaultServeMux = http.NewServeMux()

	return &server{
		t:    taskCmd,
		s:    &http.Server{},
		l:    l,
		path: path,
	}, nil
}

func (s *server) Serve() error {

	rpc.HandleHTTP()
	rpc.Register(s.t)

	return s.s.Serve(s.l)
}

func (s *server) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := s.s.Shutdown(ctx)
	return err
}
