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
	"os"
	"runtime"
	"strings"

	"github.com/dustin/go-humanize"
	"github.com/hugelgupf/go-shlex"
	"github.com/u-root/gobusybox/src/pkg/golang"
	"github.com/u-root/gobusybox/src/pkg/uflag"
	"github.com/u-root/mkuimage/uroot"
	"github.com/u-root/mkuimage/uroot/builder"
	"github.com/u-root/mkuimage/uroot/initramfs"
	"github.com/u-root/uio/ulog"
)

// multiFlag is used for flags that support multiple invocations, e.g. -files.
type multiFlag []string

func (m *multiFlag) String() string {
	return fmt.Sprint(*m)
}

// Set implements flag.Value.Set.
func (m *multiFlag) Set(value string) error {
	*m = append(*m, value)
	return nil
}

var (
	errEmptyFilesArg = errors.New("empty argument to -files")
)

// Flags for u-root builder.
var (
	build, format, tmpDir, basePath, outputPath *string
	uinitCmd, initCmd                           *string
	defaultShell                                *string
	useExistingInit                             *bool
	noCommands                                  *bool
	extraFiles                                  multiFlag
	shellbang                                   *bool
	// For the new "filepath only" logic.
	urootSourceDir *string
)

func init() {
	var sh string
	switch golang.Default().GOOS {
	case "plan9":
		sh = ""
	default:
		sh = "elvish"
	}

	build = flag.String("build", "gbb", "u-root build format (e.g. bb/gbb or binary).")
	format = flag.String("format", "cpio", "Archival format.")

	tmpDir = flag.String("tmpdir", "", "Temporary directory to put binaries in.")

	basePath = flag.String("base", "", "Base archive to add files to. By default, this is a couple of directories like /bin, /etc, etc. u-root has a default internally supplied set of files; use base=/dev/null if you don't want any base files.")
	useExistingInit = flag.Bool("useinit", false, "Use existing init from base archive (only if --base was specified).")
	outputPath = flag.String("o", "", "Path to output initramfs file.")

	initCmd = flag.String("initcmd", "init", "Symlink target for /init. Can be an absolute path or a u-root command name. Use initcmd=\"\" if you don't want the symlink.")
	uinitCmd = flag.String("uinitcmd", "", "Symlink target and arguments for /bin/uinit. Can be an absolute path or a u-root command name. Use uinitcmd=\"\" if you don't want the symlink. E.g. -uinitcmd=\"echo foobar\"")
	defaultShell = flag.String("defaultsh", sh, "Default shell. Can be an absolute path or a u-root command name. Use defaultsh=\"\" if you don't want the symlink.")

	noCommands = flag.Bool("nocmd", false, "Build no Go commands; initramfs only")

	flag.Var(&extraFiles, "files", "Additional files, directories, and binaries (with their ldd dependencies) to add to archive. Can be specified multiple times.")

	shellbang = flag.Bool("shellbang", false, "Use #! instead of symlinks for busybox")

	// Flag for the new filepath only mode. This will be required to find the u-root commands and make templates work
	// In almost every case, "." is fine.
	urootSourceDir = flag.String("uroot-source", ".", "Path to the locally checked out u-root source tree in case commands from there are desired.")
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

func main() {
	if err := checkArgs(os.Args...); err != nil {
		log.Fatal(err)
	}

	gbbOpts := &golang.BuildOpts{}
	gbbOpts.RegisterFlags(flag.CommandLine)
	// Register an alias for -go-no-strip for backwards compatibility.
	flag.CommandLine.BoolVar(&gbbOpts.NoStrip, "no-strip", false, "Build unstripped binaries")

	env := golang.Default()
	env.RegisterFlags(flag.CommandLine)
	tags := (*uflag.Strings)(&env.BuildTags)
	flag.CommandLine.Var(tags, "tags", "Go build tags -- repeat the flag for multiple values")

	flag.Parse()

	l := log.New(os.Stderr, "", log.Ltime)

	if usrc := os.Getenv("UROOT_SOURCE"); usrc != "" && *urootSourceDir == "" {
		*urootSourceDir = usrc
	}

	if env.CgoEnabled {
		l.Printf("Disabling CGO for u-root...")
		env.CgoEnabled = false
	}
	l.Printf("Build environment: %s", env)
	if env.GOOS != "linux" {
		l.Printf("GOOS is not linux. Did you mean to set GOOS=linux?")
	}

	// Main is in a separate functions so defers run on return.
	if err := Main(l, env, gbbOpts); err != nil {
		l.Fatalf("Build error: %v", err)
	}

	if stat, err := os.Stat(*outputPath); err == nil {
		l.Printf("Successfully built %q (size %d bytes -- %s).", *outputPath, stat.Size(), humanize.IBytes(uint64(stat.Size())))
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

func getReader(format string, path string) initramfs.ReadOpener {
	switch format {
	case "cpio":
		return &initramfs.CPIOFile{Path: path}
	default:
		return nil
	}
}

func getWriter(format string, path string) initramfs.WriteOpener {
	switch format {
	case "cpio":
		return &initramfs.CPIOFile{Path: path}
	case "dir":
		return &initramfs.Dir{Path: path}
	default:
		return nil
	}
}

func defaultFile(env *golang.Environ) string {
	if len(env.GOOS) == 0 || len(env.GOARCH) == 0 {
		return "/tmp/initramfs.cpio"
	}
	return fmt.Sprintf("/tmp/initramfs.%s_%s.cpio", env.GOOS, env.GOARCH)
}

// Main is a separate function so defers are run on return, which they wouldn't
// on exit.
func Main(l ulog.Logger, env *golang.Environ, buildOpts *golang.BuildOpts) error {
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

	if *outputPath == "" && *format == "cpio" {
		*outputPath = defaultFile(env)
	}
	output := getWriter(*format, *outputPath)

	var base initramfs.ReadOpener
	base = &initramfs.Archive{Archive: uroot.DefaultRamfs()}
	if *basePath != "" {
		base = getReader(*format, *basePath)
	}

	tempDir := *tmpDir
	if tempDir == "" {
		var err error
		tempDir, err = os.MkdirTemp("", "u-root")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tempDir)
	} else if _, err := os.Stat(tempDir); os.IsNotExist(err) {
		if err := os.MkdirAll(tempDir, 0o755); err != nil {
			return fmt.Errorf("temporary directory %q did not exist; tried to mkdir but failed: %v", tempDir, err)
		}
	}

	var c []uroot.Commands
	if !*noCommands {
		var b builder.Builder
		switch *build {
		case "bb", "gbb":
			b = builder.GBBBuilder{ShellBang: *shellbang}
		case "binary":
			b = builder.BinaryBuilder{}
		default:
			return fmt.Errorf("could not find builder %q", *build)
		}

		pkgs := flag.Args()
		if len(pkgs) == 0 {
			pkgs = []string{"github.com/u-root/u-root/cmds/core/*"}
		}

		c = append(c, uroot.Commands{
			Builder:   b,
			Packages:  pkgs,
			BuildOpts: buildOpts,
		})
	}

	opts := uroot.Opts{
		Env:             env,
		Commands:        c,
		UrootSource:     *urootSourceDir,
		TempDir:         tempDir,
		ExtraFiles:      extraFiles,
		OutputFile:      output,
		BaseArchive:     base,
		UseExistingInit: *useExistingInit,
		InitCmd:         *initCmd,
		DefaultShell:    *defaultShell,
	}
	uinitArgs := shlex.Split(*uinitCmd)
	if len(uinitArgs) > 0 {
		opts.UinitCmd = uinitArgs[0]
	}
	if len(uinitArgs) > 1 {
		opts.UinitArgs = uinitArgs[1:]
	}
	return uroot.CreateInitramfs(l, opts)
}
