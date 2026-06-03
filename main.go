package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	cfg, err := parseArgs(args, stderr)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	return serve(ctx, cfg, stdout)
}

func serve(ctx context.Context, cfg serverConfig, output io.Writer) error {
	mux := http.NewServeMux()
	mux.Handle("/", imageHandler{cfg: cfg})

	server := &http.Server{
		Addr:              net.JoinHostPort("", strconv.Itoa(cfg.port)),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		if output != nil {
			fmt.Fprintf(output, "imagemock listening on port %d\n", cfg.port)
		}
		errCh <- server.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return err
		}
		err := <-errCh
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

type imageHandler struct {
	cfg serverConfig
}

func (h imageHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	spec := h.cfg.resolveRequest(r)
	ct, err := contentType(spec.format)
	if err != nil {
		http.Error(w, "unsupported image format", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", h.cfg.cache.headerValue())
	if err := encodeImage(w, spec); err != nil {
		http.Error(w, "failed to encode image", http.StatusInternalServerError)
	}
}
