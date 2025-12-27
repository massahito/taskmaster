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

func newServer(socket t.Socket, taskCmd *t.TaskCmd) (*server, error) {
	l, err := net.Listen("unix", socket.Path)
	if err != nil {
		slog.Error("rpc.newServer:", "error", err.Error())
		return nil, err
	}

	err = os.Chmod(socket.Path, socket.Mode)
	if err != nil {
		slog.Error("rpc.newServer: os.Chmod", "error", err.Error())
		l.Close()
		return nil, err
	}

	err = os.Chown(socket.Path, socket.UID, socket.GID)
	if err != nil {
		slog.Error("rpc.newServer: os.Chown", "error", err.Error())
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

func (s *server) serve() error {

	rpc.DefaultServer = rpc.NewServer()

	rpc.HandleHTTP()
	err := rpc.Register(s.t)
	if err != nil {
		slog.Error("rpc.serve:", "error", err.Error())
		return err
	}

	return s.s.Serve(s.l)
}

func (s *server) shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	defer s.l.Close()

	err := s.s.Shutdown(ctx)
	if err != nil {
		slog.Error("rpc.shutdown:", "error", err.Error())
	}
	return err
}
