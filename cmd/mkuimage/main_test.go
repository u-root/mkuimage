// Copyright 2015-2018 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/u-root/gobusybox/src/pkg/golang"
	"github.com/u-root/mkuimage/cpio"
	itest "github.com/u-root/mkuimage/uimage/initramfs/test"
	"golang.org/x/sync/errgroup"
)

var twocmds = []string{
	"github.com/u-root/u-root/cmds/core/ls",
	"github.com/u-root/u-root/cmds/core/init",
}

func TestUrootCmdline(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	execPath := filepath.Join(t.TempDir(), "binary")
	// Build the stuff.
	// NoTrimPath ensures that the right Go version is used when running the tests.
	goEnv := golang.Default()
	if err := goEnv.BuildDir(wd, execPath, &golang.BuildOpts{NoStrip: true, NoTrimPath: true, ExtraArgs: []string{"-cover"}}); err != nil {
		t.Fatal(err)
	}

	gocoverdir := filepath.Join(wd, "cover")
	os.RemoveAll(gocoverdir)
	if err := os.Mkdir(gocoverdir, 0o777); err != nil && !os.IsExist(err) {
		t.Fatal(err)
	}

	samplef, err := os.CreateTemp(t.TempDir(), "u-root-test-")
	if err != nil {
		t.Fatal(err)
	}
	samplef.Close()

	sampledir := t.TempDir()
	if err = os.WriteFile(filepath.Join(sampledir, "foo"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(sampledir, "bar"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	type testCase struct {
		name       string
		env        []string
		args       []string
		err        error
		validators []itest.ArchiveValidator
	}

	noCmdTests := []testCase{
		{
			name: "include one extra file",
			args: []string{"-nocmd", "-files=/bin/bash"},
			env:  []string{"GO111MODULE=off"},
			err:  nil,
			validators: []itest.ArchiveValidator{
				itest.HasFile{Path: "bin/bash"},
			},
		},
		{
			name: "fix usage of an absolute path",
			args: []string{"-nocmd", fmt.Sprintf("-files=%s:/bin", sampledir)},
			env:  []string{"GO111MODULE=off"},
			err:  nil,
			validators: []itest.ArchiveValidator{
				itest.HasFile{Path: "/bin/foo"},
				itest.HasFile{Path: "/bin/bar"},
			},
		},
		{
			name: "include multiple extra files",
			args: []string{"-nocmd", "-files=/bin/bash", "-files=/bin/ls", fmt.Sprintf("-files=%s", samplef.Name())},
			env:  []string{"GO111MODULE=off"},
			validators: []itest.ArchiveValidator{
				itest.HasFile{Path: "bin/bash"},
				itest.HasFile{Path: "bin/ls"},
				itest.HasFile{Path: samplef.Name()},
			},
		},
		{
			name: "include one extra file with rename",
			args: []string{"-nocmd", "-files=/bin/bash:bin/bush"},
			env:  []string{"GO111MODULE=off"},
			validators: []itest.ArchiveValidator{
				itest.HasFile{Path: "bin/bush"},
			},
		},
		{
			name: "supplied file can be uinit",
			args: []string{"-nocmd", "-files=/bin/bash:bin/bash", "-uinitcmd=/bin/bash"},
			env:  []string{"GO111MODULE=off"},
			validators: []itest.ArchiveValidator{
				itest.HasFile{Path: "bin/bash"},
				itest.HasRecord{R: cpio.Symlink("bin/uinit", "bash")},
			},
		},
	}

	bareTests := []testCase{
		{
			name: "uinitcmd",
			args: []string{"-uinitcmd=echo foobar fuzz", "-defaultsh=", "github.com/u-root/u-root/cmds/core/init", "github.com/u-root/u-root/cmds/core/echo"},
			err:  nil,
			validators: []itest.ArchiveValidator{
				itest.HasRecord{R: cpio.Symlink("bin/uinit", "../bbin/echo")},
				itest.HasContent{
					Path:    "etc/uinit.flags",
					Content: "\"foobar\"\n\"fuzz\"",
				},
			},
		},
		{
			name: "binary build",
			args: []string{"-build=binary", "-defaultsh=", "github.com/u-root/u-root/cmds/core/init", "github.com/u-root/u-root/cmds/core/echo"},
			err:  nil,
			validators: []itest.ArchiveValidator{
				itest.HasFile{Path: "bin/init"},
				itest.HasFile{Path: "bin/echo"},
				itest.HasRecord{R: cpio.CharDev("dev/tty", 0o666, 5, 0)},
			},
		},
		{
			name: "hosted mode",
			args: append([]string{"-base=/dev/null", "-defaultsh=", "-initcmd="}, twocmds...),
		},
		{
			name: "AMD64 build",
			env:  []string{"GOARCH=amd64"},
			args: []string{
				"github.com/u-root/u-root/cmds/core/init",
				"github.com/u-root/u-root/cmds/core/elvish",
				"github.com/u-root/u-root/cmds/core/echo",
			},
		},
		{
			name: "ARM7 build",
			env:  []string{"GOARCH=arm", "GOARM=7"},
			args: []string{
				"github.com/u-root/u-root/cmds/core/init",
				"github.com/u-root/u-root/cmds/core/elvish",
				"github.com/u-root/u-root/cmds/core/echo",
			},
		},
		{
			name: "ARM64 build",
			env:  []string{"GOARCH=arm64"},
			args: []string{
				"github.com/u-root/u-root/cmds/core/init",
				"github.com/u-root/u-root/cmds/core/elvish",
				"github.com/u-root/u-root/cmds/core/echo",
			},
		},
		{
			name: "RISCV 64bit build",
			env:  []string{"GOARCH=riscv64"},
			args: []string{
				"github.com/u-root/u-root/cmds/core/init",
				"github.com/u-root/u-root/cmds/core/elvish",
				"github.com/u-root/u-root/cmds/core/echo",
			},
		},
	}

	for _, tt := range append(noCmdTests, bareTests...) {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			var g errgroup.Group
			var f1, f2 *os.File
			var sum1, sum2 []byte

			g.Go(func() error {
				var err error
				f1, sum1, err = buildIt(t, execPath, tt.args, tt.env, tt.err, gocoverdir)
				return err
			})

			g.Go(func() error {
				var err error
				f2, sum2, err = buildIt(t, execPath, tt.args, tt.env, tt.err, gocoverdir)
				return err
			})

			if err := g.Wait(); err != nil {
				t.Fatal(err)
			}

			a, err := itest.ReadArchive(f1.Name())
			if err != nil {
				t.Fatal(err)
			}
			for _, v := range tt.validators {
				if err := v.Validate(a); err != nil {
					t.Errorf("validator failed: %v / archive:\n%s", err, a)
				}
			}

			if !bytes.Equal(sum1, sum2) {
				t.Errorf("not reproducible, hashes don't match")
				t.Errorf("env: %v args: %v", tt.env, tt.args)
				t.Errorf("file1: %v file2: %v", f1.Name(), f2.Name())
			}
		})
	}
}

func buildIt(t *testing.T, execPath string, args, env []string, want error, gocoverdir string) (*os.File, []byte, error) {
	t.Helper()
	initramfs, err := os.CreateTemp(t.TempDir(), "u-root-")
	if err != nil {
		return nil, nil, err
	}

	// Use the u-root command outside of the $GOPATH tree to make sure it
	// still works.
	args = append([]string{"-o", initramfs.Name()}, args...)
	t.Logf("Commandline: %v mkuimage %v", strings.Join(env, " "), strings.Join(args, " "))

	c := exec.Command(execPath, args...)
	c.Env = append(os.Environ(), env...)
	c.Env = append(c.Env, golang.Default().Env()...)
	c.Env = append(c.Env, "GOCOVERDIR="+gocoverdir)
	if out, err := c.CombinedOutput(); err != want {
		return nil, nil, fmt.Errorf("Error: %v\nOutput:\n%s", err, out)
	} else if err != nil {
		return initramfs, nil, err
	}

	h1 := sha256.New()
	if _, err := io.Copy(h1, initramfs); err != nil {
		return nil, nil, err
	}
	return initramfs, h1.Sum(nil), nil
}

func TestCheckArgs(t *testing.T) {
	for _, tt := range []struct {
		name string
		args []string
		err  error
	}{
		{"-files is only arg", []string{"-files"}, errEmptyFilesArg},
		{"-files followed by -files", []string{"-files", "-files"}, errEmptyFilesArg},
		{"-files followed by any other switch", []string{"-files", "-abc"}, errEmptyFilesArg},
		{"no args", []string{}, nil},
		{"u-root alone", []string{"u-root"}, nil},
		{"u-root with -files and other args", []string{"u-root", "-files", "/bin/bash", "core"}, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := checkArgs(tt.args...); !errors.Is(err, tt.err) {
				t.Errorf("%q: got %v, want %v", tt.args, err, tt.err)
			}
		})
	}
}
