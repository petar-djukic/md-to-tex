// Copyright (c) 2026 Petar Djukic. All rights reserved. SPDX-License-Identifier: MIT

// Command md-to-tex runs the library as a service (srd010-service). The one
// subcommand, serve, listens on an address and answers the parse, convert,
// render, and health routes; the container image runs it as its entrypoint.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/petar-djukic/md-to-tex/internal/service"
)

// DefaultListen is the address serve binds when none is given (R1.1).
const DefaultListen = ":8090"

const usage = "usage: md-to-tex serve [--listen " + DefaultListen + "]"

func main() {
	config, err := parseArgs(os.Args[1:], os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := serve(ctx, config); err != nil {
		log.Fatal(err)
	}
}

type serveConfig struct {
	listen string
}

// parseArgs accepts serve and its flags and nothing else.
func parseArgs(args []string, stderr io.Writer) (serveConfig, error) {
	if len(args) < 1 || args[0] != "serve" {
		return serveConfig{}, errors.New(usage)
	}
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	flags.SetOutput(stderr)
	listen := flags.String("listen", DefaultListen, "address to listen on")
	if err := flags.Parse(args[1:]); err != nil {
		return serveConfig{}, err
	}
	if flags.NArg() != 0 {
		return serveConfig{}, fmt.Errorf("%s: unexpected argument %q", usage, flags.Arg(0))
	}
	return serveConfig{listen: *listen}, nil
}

func serve(ctx context.Context, config serveConfig) error {
	server := &http.Server{
		Addr:              config.listen,
		Handler:           service.New(service.Handler{}),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	log.Printf("md-to-tex serve: listening on %s", config.listen)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
