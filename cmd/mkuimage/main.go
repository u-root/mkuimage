// Copyright 2015-2018 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Command mkuimage builds CPIO archives with the given files and Go commands.
package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/dustin/go-humanize"
	"github.com/u-root/gobusybox/src/pkg/golang"
	"github.com/u-root/mkuimage/uimage"
	"github.com/u-root/mkuimage/uimage/mkuimage"
	"github.com/u-root/uio/llog"
)

var (
	errEmptyFilesArg = errors.New("empty argument to -files")
)

// checkArgs checks for common mistakes that cause confusion.
//  1. -files as the last argument
//  2. -files followed by any switch, indicating a shell expansion problem
//     This is usually caused by Makfiles structured as follows
//     u-root -files `which ethtool` -files `which bash`
//     if ethtool is not installed, the expansion yields
//     u-root -files -files `which bash`
//     and the rather confusing error message
//     16:14:51 Skipping /usr/bin/bash because it is not a directory
//     which, in practice, nobody understands
func checkArgs(args ...string) error {
	if len(args) == 0 {
		return nil
	}

	if args[len(args)-1] == "-files" {
		return fmt.Errorf("last argument is -files:%w", errEmptyFilesArg)
	}

	// We know the last arg is not -files; scan the arguments for -files
	// followed by a switch.
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "-files" && args[i+1][0] == '-' {
			return fmt.Errorf("-files argument %d is followed by a switch: %w", i, errEmptyFilesArg)
		}
	}

	return nil
}

func main() {
	if err := checkArgs(os.Args...); err != nil {
		log.Fatal(err)
	}

	var sh string
	if golang.Default().GOOS != "plan9" {
		sh = "gosh"
	}

	env := golang.Default(golang.DisableCGO())
	f := &mkuimage.Flags{
		Commands: mkuimage.CommandFlags{
			Builder:   "bb",
			BuildOpts: &golang.BuildOpts{},
		},
		Init:          "init",
		Shell:         sh,
		ArchiveFormat: "cpio",
		OutputFile:    defaultFile(env),
	}
	f.RegisterFlags(flag.CommandLine)

	l := llog.Default()
	l.RegisterVerboseFlag(flag.CommandLine, "v", slog.LevelDebug)
	flag.Parse()

	m := []uimage.Modifier{
		uimage.WithReplaceEnv(env),
		uimage.WithBaseArchive(uimage.DefaultRamfs()),
		uimage.WithCPIOOutput(defaultFile(env)),
	}
	if err := mkuimage.CreateUimage(l, m, f, flag.Args()); err != nil {
		l.Errorf("mkuimage error: %v", err)
		os.Exit(1)
	}

	if stat, err := os.Stat(f.OutputFile); err == nil && f.ArchiveFormat == "cpio" {
		l.Infof("Successfully built %q (size %d bytes -- %s).", f.OutputFile, stat.Size(), humanize.IBytes(uint64(stat.Size())))
	}
}

func defaultFile(env *golang.Environ) string {
	if len(env.GOOS) == 0 || len(env.GOARCH) == 0 {
		return "/tmp/initramfs.cpio"
	}
	return fmt.Sprintf("/tmp/initramfs.%s_%s.cpio", env.GOOS, env.GOARCH)
}
