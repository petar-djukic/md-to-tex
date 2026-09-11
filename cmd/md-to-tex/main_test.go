// Copyright (c) 2026 Petar Djukic. All rights reserved. SPDX-License-Identifier: MIT

package main

import (
	"io"
	"strings"
	"testing"
	"time"

	"github.com/petar-djukic/md-to-tex/internal/service"
)

// The binary has one subcommand and stated bounds (srd010-service R1.1, R1.5).
func TestServeIsTheOnlySubcommand(t *testing.T) {
	for _, args := range [][]string{{}, {"parse"}, {"serve", "extra"}} {
		if _, err := parseArgs(args, io.Discard); err == nil {
			t.Errorf("args %v were accepted", args)
		}
	}
	config, err := parseArgs([]string{"serve"}, io.Discard)
	if err != nil || config.listen != DefaultListen || DefaultListen != ":8090" {
		t.Fatalf("serve defaults = %+v, %v", config, err)
	}
	config, err = parseArgs([]string{"serve", "--listen", "127.0.0.1:0"}, io.Discard)
	if err != nil || config.listen != "127.0.0.1:0" {
		t.Fatalf("serve --listen = %+v, %v", config, err)
	}
	if !strings.Contains(usage, "serve") || service.MaxBodyBytes != 16<<20 || service.RenderTimeout != 3*time.Minute {
		t.Fatalf("usage %q, body bound %d, render bound %s", usage, service.MaxBodyBytes, service.RenderTimeout)
	}
}
