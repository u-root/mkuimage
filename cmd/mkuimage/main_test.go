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
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/u-root/mkuimage/cpio"
	"github.com/u-root/mkuimage/testutil"
	itest "github.com/u-root/mkuimage/uroot/initramfs/test"
)

var twocmds = []string{
	"github.com/u-root/u-root/cmds/core/ls",
	"github.com/u-root/u-root/cmds/core/init",
}

func TestUrootCmdline(t *testing.T) {
	samplef, err := os.CreateTemp("", "u-root-test-")
	if err != nil {
		t.Fatal(err)
	}
	samplef.Close()
	defer os.RemoveAll(samplef.Name())
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
			name: "MIPS build",
			env:  []string{"GOARCH=mips"},
			args: []string{
				"github.com/u-root/u-root/cmds/core/init",
				"github.com/u-root/u-root/cmds/core/elvish",
				"github.com/u-root/u-root/cmds/core/echo",
			},
		},
		{
			name: "MIPSLE build",
			env:  []string{"GOARCH=mipsle"},
			args: []string{
				"github.com/u-root/u-root/cmds/core/init",
				"github.com/u-root/u-root/cmds/core/elvish",
				"github.com/u-root/u-root/cmds/core/echo",
			},
		},
		{
			name: "MIPS64 build",
			env:  []string{"GOARCH=mips64"},
			args: []string{
				"github.com/u-root/u-root/cmds/core/init",
				"github.com/u-root/u-root/cmds/core/elvish",
				"github.com/u-root/u-root/cmds/core/echo",
			},
		},
		{
			name: "MIPS64LE build",
			env:  []string{"GOARCH=mips64le"},
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
			name: "386 (32 bit) build",
			env:  []string{"GOARCH=386"},
			args: []string{
				"github.com/u-root/u-root/cmds/core/init",
				"github.com/u-root/u-root/cmds/core/elvish",
				"github.com/u-root/u-root/cmds/core/echo",
			},
		},
		{
			name: "Power 64bit build",
			env:  []string{"GOARCH=ppc64le"},
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
	var bbTests []testCase
	for _, test := range bareTests {
		gbbTest := test
		gbbTest.name += " gbb-gomodule"
		gbbTest.args = append([]string{"-build=gbb"}, gbbTest.args...)
		gbbTest.env = append(gbbTest.env, "GO111MODULE=on")

		bbTests = append(bbTests, gbbTest)
	}

	for _, tt := range append(noCmdTests, bbTests...) {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			delFiles := true
			var (
				f1, f2     *os.File
				sum1, sum2 []byte
				errs       [2]error
				wg         = &sync.WaitGroup{}
				removeMu   sync.Mutex
				remove     []string
			)

			wg.Add(2)
			go func() {
				defer wg.Done()
				f1, sum1, err = buildIt(t, tt.args, tt.env, tt.err)
				if err != nil {
					errs[0] = err
					return
				}

				a, err := itest.ReadArchive(f1.Name())
				if err != nil {
					errs[0] = err
					return
				}

				removeMu.Lock()
				remove = append(remove, f1.Name())
				removeMu.Unlock()
				for _, v := range tt.validators {
					if err := v.Validate(a); err != nil {
						t.Errorf("validator failed: %v / archive:\n%s", err, a)
					}
				}
			}()

			go func() {
				defer wg.Done()
				var err error
				f2, sum2, err = buildIt(t, tt.args, tt.env, tt.err)
				if err != nil {
					errs[1] = err
					return
				}
				removeMu.Lock()
				remove = append(remove, f2.Name())
				removeMu.Unlock()
			}()

			wg.Wait()
			defer func() {
				if delFiles {
					for _, n := range remove {
						os.RemoveAll(n)
					}
				}
			}()
			if errs[0] != nil {
				t.Error(errs[0])
				return
			}
			if errs[1] != nil {
				t.Error(errs[1])
				return
			}
			if !bytes.Equal(sum1, sum2) {
				delFiles = false
				t.Errorf("not reproducible, hashes don't match")
				t.Errorf("env: %v args: %v", tt.env, tt.args)
				t.Errorf("file1: %v file2: %v", f1.Name(), f2.Name())
			}
		})
	}
}

func buildIt(t *testing.T, args, env []string, want error) (*os.File, []byte, error) {
	t.Helper()
	f, err := os.CreateTemp("", "u-root-")
	if err != nil {
		return nil, nil, err
	}
	// Use the u-root command outside of the $GOPATH tree to make sure it
	// still works.
	arg := append([]string{"-o", f.Name()}, args...)
	c := testutil.Command(t, arg...)
	t.Logf("Commandline: %v u-root %v", strings.Join(env, " "), strings.Join(arg, " "))
	c.Env = append(c.Env, env...)
	if out, err := c.CombinedOutput(); err != want {
		return nil, nil, fmt.Errorf("Error: %v\nOutput:\n%s", err, out)
	} else if err != nil {
		h1 := sha256.New()
		if _, err := io.Copy(h1, f); err != nil {
			return nil, nil, err
		}
		return f, h1.Sum(nil), nil
	}
	return f, nil, nil
}

func TestMain(m *testing.M) {
	testutil.Run(m, main)
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
