// Copyright 2017 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package testutil has utilities to test Go commands.
package testutil

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/u-root/gobusybox/src/pkg/golang"
)

var binary string

// Command returns an exec.Cmd appropriate for testing the u-root command.
//
// Command decides which executable to call based on environment variables:
// - EXECPATH="executable args" overrides any other test subject.
// - UROOT_TEST_BUILD=1 will force compiling the u-root command in question.
func Command(tb testing.TB, args ...string) *exec.Cmd {
	tb.Helper()
	// If EXECPATH is set, just use that.
	execPath := os.Getenv("EXECPATH")
	if len(execPath) > 0 {
		exe := strings.Split(os.Getenv("EXECPATH"), " ")
		return exec.Command(exe[0], append(exe[1:], args...)...)
	}

	// Should be cached by Run if os.Executable is going to fail.
	if len(binary) > 0 {
		tb.Logf("binary: %v", binary)
		return exec.Command(binary, args...)
	}

	execPath, err := os.Executable()
	if err != nil {
		// Run should have prevented this case by caching something in
		// `binary`.
		tb.Fatal("You must call testutil.Run() in your TestMain.")
	}

	c := exec.Command(execPath, args...)
	c.Env = append(c.Env, append(os.Environ(), "UROOT_CALL_MAIN=1")...)
	return c
}

func run(m *testing.M, mainFn func()) int {
	// UROOT_CALL_MAIN=1 /proc/self/exe should be the same as just running
	// the command we are testing.
	if len(os.Getenv("UROOT_CALL_MAIN")) > 0 {
		mainFn()
		return 0
	}

	// Normally, /proc/self/exe (and equivalents) are used to test u-root
	// commands.
	//
	// Such a symlink isn't available on Plan 9, OS X, or FreeBSD. On these
	// systems, we compile the u-root command in question on the fly
	// instead.
	//
	// Here, we decide whether to compile or not and cache the executable.
	// Do this here, so that when m.Run() returns, we can remove the
	// executable using the functor returned.
	_, err := os.Executable()
	if err != nil || len(os.Getenv("UROOT_TEST_BUILD")) > 0 {
		// We can't find ourselves? Probably FreeBSD or something. Try to go
		// build the command.
		//
		// This is NOT build-system-independent, and hence the fallback.
		tmpDir, err := os.MkdirTemp("", "uroot-build")
		if err != nil {
			log.Print(err)
			return 1
		}
		defer os.RemoveAll(tmpDir)

		wd, err := os.Getwd()
		if err != nil {
			log.Print(err)
			return 1
		}

		execPath := filepath.Join(tmpDir, "binary")
		// Build the stuff.
		if err := golang.Default().BuildDir(wd, execPath, nil); err != nil {
			log.Print(err)
			return 1
		}

		// Cache dat.
		binary = execPath
	}

	return m.Run()
}

// Run sets up necessary commands to be compiled, if necessary, and calls
// m.Run.
func Run(m *testing.M, mainFn func()) {
	os.Exit(run(m, mainFn))
}
