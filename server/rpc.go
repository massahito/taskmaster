package server

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"time"

	t "github.com/massahito/taskmaster"
)

type server struct {
	t    *t.TaskCmd
	s    *http.Server
	l    net.Listener
	path string
}

func NewServer(socket t.Socket, taskCmd *t.TaskCmd) (*server, error) {
	l, err := net.Listen("unix", socket.Path)
	if err != nil {
		slog.Error("rpc.NewServer:", "error", err.Error())
		return nil, err
	}

	err = os.Chmod(socket.Path, socket.Mode)
	if err != nil {
		slog.Error("rpc.NewServer: os.Chmod", "error", err.Error())
		l.Close()
		return nil, err
	}

	err = os.Chown(socket.Path, socket.UID, socket.GID)
	if err != nil {
		slog.Error("rpc.NewServer: os.Chown", "error", err.Error())
		l.Close()
		return nil, err
	}

	http.DefaultServeMux = http.NewServeMux()

	return &server{
		t:    taskCmd,
		s:    &http.Server{},
		l:    l,
		path: socket.Path,
	}, nil
}

func (s *server) Serve() error {

	rpc.DefaultServer = rpc.NewServer()

	rpc.HandleHTTP()
	err := rpc.Register(s.t)
	if err != nil {
		slog.Error("rpc.Serve:", "error", err.Error())
		return err
	}

	return s.s.Serve(s.l)
}

func (s *server) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	defer s.l.Close()

	err := s.s.Shutdown(ctx)
	if err != nil {
		slog.Error("rpc.Shutdown:", "error", err.Error())
	}
	return err
}
