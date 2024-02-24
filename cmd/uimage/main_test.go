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
	"regexp"
	"strings"
	"syscall"
	"testing"

	"github.com/u-root/gobusybox/src/pkg/golang"
	"github.com/u-root/mkuimage/cpio"
	itest "github.com/u-root/mkuimage/uimage/initramfs/test"
	"golang.org/x/sync/errgroup"
)

func hasTempDir(t *testing.T, output string) {
	t.Helper()
	tempDir := regexp.MustCompile(`Keeping temp dir (.+)`).FindStringSubmatch(output)
	if tempDir == nil {
		t.Errorf("Keeping temp dir not found in output")
		return
	}
	if fi, err := os.Stat(tempDir[1]); err != nil {
		t.Error(err)
	} else if !fi.IsDir() {
		t.Errorf("Stat(%s) = %v, want directory", tempDir[1], fi)
	}
}

func dirExists(name string) func(t *testing.T, output string) {
	return func(t *testing.T, output string) {
		t.Helper()
		if fi, err := os.Stat(name); err != nil {
			t.Error(err)
		} else if !fi.IsDir() {
			t.Errorf("Stat(%s) = %v, want directory", name, fi)
		}
	}
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
	tempDir := filepath.Join(t.TempDir(), "tempdir")

	type testCase struct {
		name       string
		env        []string
		args       []string
		exitCode   int
		validators []itest.ArchiveValidator
		wantOutput func(*testing.T, string)
	}

	tests := []testCase{
		{
			name: "include one extra file",
			args: []string{"make", "-nocmd", "-files=/bin/bash"},
			env:  []string{"GO111MODULE=off"},
			validators: []itest.ArchiveValidator{
				itest.HasFile{Path: "bin/bash"},
			},
		},
		{
			name: "fix usage of an absolute path",
			args: []string{"make", "-nocmd", fmt.Sprintf("-files=%s:/bin", sampledir)},
			env:  []string{"GO111MODULE=off"},
			validators: []itest.ArchiveValidator{
				itest.HasFile{Path: "/bin/foo"},
				itest.HasFile{Path: "/bin/bar"},
			},
		},
		{
			name: "include multiple extra files",
			args: []string{"make", "-nocmd", "-files=/bin/bash", "-files=/bin/ls", fmt.Sprintf("-files=%s", samplef.Name())},
			env:  []string{"GO111MODULE=off"},
			validators: []itest.ArchiveValidator{
				itest.HasFile{Path: "bin/bash"},
				itest.HasFile{Path: "bin/ls"},
				itest.HasFile{Path: samplef.Name()},
			},
		},
		{
			name: "include one extra file with rename",
			args: []string{"make", "-nocmd", "-files=/bin/bash:bin/bush"},
			env:  []string{"GO111MODULE=off"},
			validators: []itest.ArchiveValidator{
				itest.HasFile{Path: "bin/bush"},
			},
		},
		{
			name: "supplied file can be uinit",
			args: []string{"make", "-nocmd", "-files=/bin/bash:bin/bash", "-uinitcmd=/bin/bash"},
			env:  []string{"GO111MODULE=off"},
			validators: []itest.ArchiveValidator{
				itest.HasFile{Path: "bin/bash"},
				itest.HasRecord{R: cpio.Symlink("bin/uinit", "bash")},
			},
		},
		{
			name: "uinitcmd",
			args: []string{"make", "-uinitcmd=echo foobar fuzz", "-defaultsh=", "github.com/u-root/u-root/cmds/core/init", "github.com/u-root/u-root/cmds/core/echo"},
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
			args: []string{
				"make",
				"-build=binary",
				"-defaultsh=",
				"github.com/u-root/u-root/cmds/core/init",
				"github.com/u-root/u-root/cmds/core/echo",
			},
			validators: []itest.ArchiveValidator{
				itest.HasFile{Path: "bin/init"},
				itest.HasFile{Path: "bin/echo"},
				itest.HasRecord{R: cpio.CharDev("dev/tty", 0o666, 5, 0)},
			},
		},
		{
			name: "hosted mode",
			args: []string{
				"make",
				"-base=/dev/null",
				"-defaultsh=",
				"-initcmd=",
				"github.com/u-root/u-root/cmds/core/ls",
				"github.com/u-root/u-root/cmds/core/init",
			},
		},
		{
			name: "AMD64 build",
			env:  []string{"GOARCH=amd64"},
			args: []string{
				"make",
				"-defaultsh=echo",
				"github.com/u-root/u-root/cmds/core/echo",
				"github.com/u-root/u-root/cmds/core/init",
			},
		},
		{
			name: "AMD64 build with temp dir",
			env:  []string{"GOARCH=amd64"},
			args: []string{
				"make",
				"--keep-tmp-dir",
				"--defaultsh=echo",
				"github.com/u-root/u-root/cmds/core/echo",
				"github.com/u-root/u-root/cmds/core/init",
			},
			exitCode:   1,
			wantOutput: hasTempDir,
		},
		{
			name: "ARM7 build",
			env:  []string{"GOARCH=arm", "GOARM=7"},
			args: []string{
				"make",
				"-defaultsh=",
				"github.com/u-root/u-root/cmds/core/init",
				"github.com/u-root/u-root/cmds/core/echo",
			},
		},
		{
			name: "ARM64 build",
			env:  []string{"GOARCH=arm64"},
			args: []string{
				"make",
				"-defaultsh=",
				"github.com/u-root/u-root/cmds/core/init",
				"github.com/u-root/u-root/cmds/core/echo",
			},
		},
		{
			name: "RISCV 64bit build",
			env:  []string{"GOARCH=riscv64"},
			args: []string{
				"make",
				"-defaultsh=",
				"github.com/u-root/u-root/cmds/core/init",
				"github.com/u-root/u-root/cmds/core/echo",
			},
		},
		{
			name: "build invalid",
			args: []string{
				"make",
				"-build=source",
				"github.com/u-root/u-root/cmds/core/init",
				"github.com/u-root/u-root/cmds/core/echo",
			},
			exitCode: 1,
		},
		{
			name: "arch invalid preserves temp dir",
			env:  []string{"GOARCH=doesnotexist"},
			args: []string{
				"make",
				"--defaultsh=echo",
				"github.com/u-root/u-root/cmds/core/echo",
				"github.com/u-root/u-root/cmds/core/init",
			},
			exitCode:   1,
			wantOutput: hasTempDir,
		},
		{
			name: "specify temp dir",
			args: []string{
				"make",
				"--tmp-dir=" + tempDir,
				"github.com/u-root/u-root/cmds/core/echo",
				"github.com/u-root/u-root/cmds/core/init",
			},
			exitCode:   1,
			wantOutput: dirExists(tempDir),
		},
		{
			name: "template config",
			args: []string{
				"make",
				"-config-file=./testdata/test-config.yaml",
				"-v",
				"-config=coreconf",
			},
			validators: []itest.ArchiveValidator{
				itest.HasRecord{R: cpio.CharDev("dev/tty", 0o666, 5, 0)},
				itest.HasFile{Path: "bbin/bb"},
				itest.HasRecord{R: cpio.Symlink("bbin/echo", "bb")},
				itest.HasRecord{R: cpio.Symlink("bbin/ip", "bb")},
				itest.HasRecord{R: cpio.Symlink("bbin/init", "bb")},
				itest.HasRecord{R: cpio.Symlink("init", "bbin/init")},
				itest.HasRecord{R: cpio.Symlink("bin/sh", "../bbin/echo")},
				itest.HasRecord{R: cpio.Symlink("bin/uinit", "../bbin/echo")},
				itest.HasRecord{R: cpio.Symlink("bin/defaultsh", "../bbin/echo")},
				itest.HasContent{
					Path:    "etc/uinit.flags",
					Content: "\"script.sh\"",
				},
			},
		},
		{
			name: "template command",
			args: []string{
				"make",
				"-config-file=./testdata/test-config.yaml",
				"-v",
				"core",
			},
			validators: []itest.ArchiveValidator{
				itest.HasRecord{R: cpio.CharDev("dev/tty", 0o666, 5, 0)},
				itest.HasFile{Path: "bbin/bb"},
				itest.HasRecord{R: cpio.Symlink("bbin/echo", "bb")},
				itest.HasRecord{R: cpio.Symlink("bbin/ip", "bb")},
				itest.HasRecord{R: cpio.Symlink("bbin/init", "bb")},
			},
		},
		{
			name:     "template config not found",
			args:     []string{"make", "-config-file=./testdata/test-config.yaml", "-v", "-config=foobar"},
			exitCode: 1,
		},
		{
			name:     "builder not found",
			args:     []string{"make", "-v", "build=source"},
			exitCode: 1,
		},
		{
			name:     "template file not found",
			args:     []string{"make", "-v", "-config-file=./testdata/doesnotexist"},
			exitCode: 1,
		},
		{
			name:     "config not found with no default template",
			args:     []string{"make", "-v", "-config=foo"},
			exitCode: 1,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			var g errgroup.Group
			var f1, f2 *os.File
			var out string
			var sum1, sum2 []byte

			g.Go(func() error {
				var err error
				f1, out, sum1, err = buildIt(t, execPath, tt.args, tt.env, gocoverdir)
				return err
			})

			g.Go(func() error {
				var err error
				f2, _, sum2, err = buildIt(t, execPath, tt.args, tt.env, gocoverdir)
				return err
			})

			err := g.Wait()
			if tt.wantOutput != nil {
				tt.wantOutput(t, out)
			}

			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				if ec := exitErr.Sys().(syscall.WaitStatus).ExitStatus(); ec != tt.exitCode {
					t.Errorf("uimage exit code = %d, want %d", ec, tt.exitCode)
				}
				return
			} else if err != nil {
				t.Errorf("uimage failed: %v", err)
				return
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

func buildIt(t *testing.T, execPath string, args, env []string, gocoverdir string) (*os.File, string, []byte, error) {
	t.Helper()
	initramfs, err := os.CreateTemp(t.TempDir(), "u-root-")
	if err != nil {
		return nil, "", nil, err
	}

	// Use the u-root command outside of the $GOPATH tree to make sure it
	// still works.
	if len(args) > 0 {
		args = append(append([]string{args[0]}, "-o", initramfs.Name()), args[1:]...)
	}
	t.Logf("Commandline: %v uimage %v", strings.Join(env, " "), strings.Join(args, " "))

	c := exec.Command(execPath, args...)
	c.Env = append(os.Environ(), env...)
	c.Env = append(c.Env, "GOCOVERDIR="+gocoverdir)
	out, err := c.CombinedOutput()
	t.Logf("Output:\n%s", out)
	if err != nil {
		return nil, string(out), nil, err
	}

	h1 := sha256.New()
	if _, err := io.Copy(h1, initramfs); err != nil {
		return nil, string(out), nil, err
	}
	return initramfs, string(out), h1.Sum(nil), nil
}
