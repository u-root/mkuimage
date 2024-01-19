// Copyright 2015-2018 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Command mkuimage builds CPIO archives with the given files and Go commands.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/u-root/gobusybox/src/pkg/golang"
	"github.com/u-root/mkuimage/uroot"
	"github.com/u-root/mkuimage/uroot/initramfs"
	"github.com/u-root/uio/ulog"
)

var (
	errEmptyFilesArg = errors.New("empty argument to -files")
)

// Flags for u-root builder.
var (
	statsOutputPath *string
	statsLabel      *string
)

func init() {
	statsOutputPath = flag.String("stats-output-path", "", "Write build stats to this file (JSON)")
	statsLabel = flag.String("stats-label", "", "Use this statsLabel when writing stats")
}

type buildStats struct {
	Label      string  `json:"label,omitempty"`
	Time       int64   `json:"time"`
	Duration   float64 `json:"duration"`
	OutputSize int64   `json:"output_size"`
}

func writeBuildStats(stats buildStats, path string) error {
	var allStats []buildStats
	data, err := os.ReadFile(*statsOutputPath)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &allStats); err != nil {
		return err
	}
	found := false
	for i, s := range allStats {
		if s.Label == stats.Label {
			allStats[i] = stats
			found = true
			break
		}
	}
	if !found {
		allStats = append(allStats, stats)
		sort.Slice(allStats, func(i, j int) bool {
			return strings.Compare(allStats[i].Label, allStats[j].Label) == -1
		})
	}
	data, err = json.MarshalIndent(allStats, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(*statsOutputPath, data, 0o644)
}

func generateLabel(env *golang.Environ) string {
	var baseCmds []string
	if len(flag.Args()) > 0 {
		// Use the last component of the name to keep the label short
		for _, e := range flag.Args() {
			baseCmds = append(baseCmds, path.Base(e))
		}
	} else {
		baseCmds = []string{"core"}
	}
	return fmt.Sprintf("%s-%s-%s", env.GOOS, env.GOARCH, strings.Join(baseCmds, "_"))
}

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

func defaultShell() string {
	switch golang.Default().GOOS {
	case "plan9":
		return ""
	default:
		return "elvish"
	}
}

func defaultOutputFile(env *golang.Environ) string {
	if env.GOOS == "" || env.GOARCH == "" {
		return ""
	}
	return fmt.Sprintf("/tmp/initramfs.%s_%s.cpio", env.GOOS, env.GOARCH)
}

func main() {
	if err := checkArgs(os.Args...); err != nil {
		log.Fatal(err)
	}

	env := golang.Default()
	opts := uroot.Opts{
		Env:           env,
		UrootSource:   os.Getenv("UROOT_SOURCE"),
		ArchiveFormat: initramfs.FormatCPIO,
		InitCmd:       "init",
		DefaultShell:  defaultShell(),
		OutputFile:    defaultOutputFile(env),
	}
	opts.RegisterFlags(flag.CommandLine)
	commands := uroot.CommandFlags{}
	commands.RegisterFlags(flag.CommandLine)

	flag.Parse()

	l := log.New(os.Stderr, "", log.Ltime)

	if env.CgoEnabled {
		l.Printf("Disabling CGO for u-root...")
		env.CgoEnabled = false
	}
	l.Printf("Build environment: %s", env)
	if env.GOOS != "linux" {
		l.Printf("GOOS is not linux. Did you mean to set GOOS=linux?")
	}

	c, err := commands.Commands(flag.Args()...)
	if err != nil {
		l.Fatalf("Error figuring out Go commands to build: %v", err)
	}
	opts.Commands = c

	start := time.Now()

	// Main is in a separate functions so defers run on return.
	if err := Main(l, env, opts); err != nil {
		l.Fatalf("Build error: %v", err)
	}

	elapsed := time.Since(start)

	stats := buildStats{
		Label:    *statsLabel,
		Time:     start.Unix(),
		Duration: float64(elapsed.Milliseconds()) / 1000,
	}
	if stats.Label == "" {
		stats.Label = generateLabel(env)
	}
	if stat, err := os.Stat(opts.OutputFile); err == nil && stat.ModTime().After(start) {
		l.Printf("Successfully built %q (size %d).", opts.OutputFile, stat.Size())
		stats.OutputSize = stat.Size()
		if *statsOutputPath != "" {
			if err := writeBuildStats(stats, *statsOutputPath); err == nil {
				l.Printf("Wrote stats to %q (label %q)", *statsOutputPath, stats.Label)
			} else {
				l.Printf("Failed to write stats to %s: %v", *statsOutputPath, err)
			}
		}
	}
}

var recommendedVersions = []string{
	"go1.20",
	"go1.21",
}

func isRecommendedVersion(v string) bool {
	for _, r := range recommendedVersions {
		if strings.HasPrefix(v, r) {
			return true
		}
	}
	return false
}

// Main is a separate function so defers are run on return, which they wouldn't
// on exit.
func Main(l ulog.Logger, env *golang.Environ, opts uroot.Opts) error {
	v, err := env.Version()
	if err != nil {
		l.Printf("Could not get environment's Go version, using runtime's version: %v", err)
		v = runtime.Version()
	}
	if !isRecommendedVersion(v) {
		l.Printf(`WARNING: You are not using one of the recommended Go versions (have = %s, recommended = %v).
			Some packages may not compile.
			Go to https://golang.org/doc/install to find out how to install a newer version of Go,
			or use https://godoc.org/golang.org/dl/%s to install an additional version of Go.`,
			v, recommendedVersions, recommendedVersions[0])
	}
	return uroot.CreateInitramfs(l, opts)
}
