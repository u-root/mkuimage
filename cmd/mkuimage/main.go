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
	"runtime"
	"strings"

	"github.com/dustin/go-humanize"
	"github.com/u-root/gobusybox/src/pkg/golang"
	"github.com/u-root/mkuimage/uimage"
	"github.com/u-root/mkuimage/uimage/templates"
	"github.com/u-root/mkuimage/uimage/uflags"
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
	log.SetFlags(log.Ltime)
	if err := checkArgs(os.Args...); err != nil {
		log.Fatal(err)
	}

	env := golang.Default(golang.DisableCGO())
	f := &uflags.Flags{
		Commands:      uflags.CommandFlags{Builder: "bb"},
		ArchiveFormat: "cpio",
		OutputFile:    defaultFile(env),
	}
	f.RegisterFlags(flag.CommandLine)

	l := llog.Default()
	l.RegisterVerboseFlag(flag.CommandLine, "v", slog.LevelDebug)

	tc := &uflags.TemplateFlags{}
	tc.RegisterFlags(flag.CommandLine)
	flag.Parse()

	tpl, err := tc.Get()
	if err != nil {
		l.Errorf("Failed to get template: %v", err)
		os.Exit(1)
	}

	// Set defaults.
	m := []uimage.Modifier{
		uimage.WithReplaceEnv(env),
		uimage.WithBaseArchive(uimage.DefaultRamfs()),
		uimage.WithCPIOOutput(defaultFile(env)),
	}

	// realMain is in a separate functions so defers run on return.
	if err := realMain(l, m, tpl, f, tc.Config); err != nil {
		l.Errorf("Build error: %v", err)
		os.Exit(1)
	}

	if stat, err := os.Stat(f.OutputFile); err == nil && f.ArchiveFormat == "cpio" {
		l.Infof("Successfully built %q (size %d bytes -- %s).", f.OutputFile, stat.Size(), humanize.IBytes(uint64(stat.Size())))
	}
}

var recommendedVersions = []string{
	"go1.20",
	"go1.21",
	"go1.22",
}

func isRecommendedVersion(v string) bool {
	for _, r := range recommendedVersions {
		if strings.HasPrefix(v, r) {
			return true
		}
	}
	return false
}

func defaultFile(env *golang.Environ) string {
	if len(env.GOOS) == 0 || len(env.GOARCH) == 0 {
		return "/tmp/initramfs.cpio"
	}
	return fmt.Sprintf("/tmp/initramfs.%s_%s.cpio", env.GOOS, env.GOARCH)
}

func uimageOpts(l *llog.Logger, m []uimage.Modifier, tpl *templates.Templates, f *uflags.Flags, conf string, cmds []string) (*uimage.Opts, error) {
	// Evaluate template first -- template settings may always be
	// appended/overridden by further flag-based settings.
	if conf != "" {
		mods, err := tpl.Uimage(conf)
		if err != nil {
			return nil, err
		}
		l.Debugf("Config: %#v", tpl.Configs[conf])
		m = append(m, mods...)
	}

	// Expand command templates.
	if tpl != nil {
		cmds = tpl.CommandsFor(cmds...)
	}

	more, err := f.Modifiers(cmds...)
	if err != nil {
		return nil, err
	}
	return uimage.OptionsFor(append(m, more...)...)
}

func realMain(l *llog.Logger, base []uimage.Modifier, tpl *templates.Templates, f *uflags.Flags, conf string) error {
	if f.TempDir == "" {
		var err error
		f.TempDir, err = os.MkdirTemp("", "u-root")
		if err != nil {
			return err
		}
		if f.KeepTempDir {
			defer func() {
				l.Infof("Keeping temp dir %s", f.TempDir)
			}()
		} else {
			defer os.RemoveAll(f.TempDir)
		}
	} else if _, err := os.Stat(f.TempDir); os.IsNotExist(err) {
		if err := os.MkdirAll(f.TempDir, 0o755); err != nil {
			return fmt.Errorf("temporary directory %q did not exist; tried to mkdir but failed: %v", f.TempDir, err)
		}
	}

	opts, err := uimageOpts(l, base, tpl, f, conf, flag.Args())
	if err != nil {
		return err
	}

	env := opts.Env
	l.Infof("Build environment: %s", env)
	if env.GOOS != "linux" {
		l.Warnf("GOOS is not linux. Did you mean to set GOOS=linux?")
	}

	v, err := env.Version()
	if err != nil {
		l.Infof("Could not get environment's Go version, using runtime's version: %v", err)
		v = runtime.Version()
	}
	if !isRecommendedVersion(v) {
		l.Warnf(`You are not using one of the recommended Go versions (have = %s, recommended = %v).
			Some packages may not compile.
			Go to https://golang.org/doc/install to find out how to install a newer version of Go,
			or use https://godoc.org/golang.org/dl/%s to install an additional version of Go.`,
			v, recommendedVersions, recommendedVersions[0])
	}
	return opts.Create(l)
}
