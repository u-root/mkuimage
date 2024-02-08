// Copyright 2018 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package uroot

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/u-root/gobusybox/src/pkg/golang"
	"github.com/u-root/mkuimage/cpio"
	"github.com/u-root/mkuimage/uroot/builder"
	itest "github.com/u-root/mkuimage/uroot/initramfs/test"
	"github.com/u-root/uio/ulog/ulogtest"
)

type inMemArchive struct {
	*cpio.Archive
}

// Finish implements initramfs.Writer.Finish.
func (inMemArchive) Finish() error { return nil }

func TestCreateInitramfs(t *testing.T) {
	dir := t.TempDir()
	syscall.Umask(0)

	urootpath, err := filepath.Abs("../../")
	if err != nil {
		t.Fatalf("failure to set up test: %v", err)
	}

	tmp777 := filepath.Join(dir, "tmp777")
	_ = os.MkdirAll(tmp777, 0o777)
	tmp400 := filepath.Join(dir, "tmp400")
	_ = os.MkdirAll(tmp400, 0o400)

	somefile := filepath.Join(dir, "somefile")
	somefile2 := filepath.Join(dir, "somefile2")
	_ = os.WriteFile(somefile, []byte("foobar"), 0o777)
	_ = os.WriteFile(somefile2, []byte("spongebob"), 0o777)

	cwd, _ := os.Getwd()

	l := ulogtest.Logger{TB: t}

	for i, tt := range []struct {
		name       string
		opts       Opts
		errs       []error
		validators []itest.ArchiveValidator
	}{
		{
			name: "BB archive",
			opts: Opts{
				Env:          golang.Default(golang.DisableCGO()),
				TempDir:      dir,
				InitCmd:      "init",
				DefaultShell: "ls",
				UrootSource:  urootpath,
				Commands: []Commands{
					{
						Builder: builder.Busybox,
						Packages: []string{
							"github.com/u-root/u-root/cmds/core/init",
							"github.com/u-root/u-root/cmds/core/ls",
						},
					},
				},
			},
			validators: []itest.ArchiveValidator{
				itest.HasFile{Path: "bbin/bb"},
				itest.HasRecord{R: cpio.Symlink("bbin/init", "bb")},
				itest.HasRecord{R: cpio.Symlink("bbin/ls", "bb")},
				itest.HasRecord{R: cpio.Symlink("bin/defaultsh", "../bbin/ls")},
				itest.HasRecord{R: cpio.Symlink("bin/sh", "../bbin/ls")},
			},
		},
		{
			name: "no temp dir",
			opts: Opts{
				InitCmd:      "init",
				DefaultShell: "",
			},
			errs: []error{os.ErrNotExist},
			validators: []itest.ArchiveValidator{
				itest.IsEmpty{},
			},
		},
		{
			name: "no commands",
			opts: Opts{
				TempDir: dir,
			},
			validators: []itest.ArchiveValidator{
				itest.MissingFile{Path: "bbin/bb"},
			},
		},
		{
			name: "files",
			opts: Opts{
				TempDir: dir,
				ExtraFiles: []string{
					somefile + ":etc/somefile",
					somefile2 + ":etc/somefile2",
					somefile,
					// Empty is ignored.
					"",
					"uroot_test.go",
					filepath.Join(cwd, "uroot_test.go"),
					// Parent directory is created.
					somefile + ":somedir/somefile",
				},
			},
			validators: []itest.ArchiveValidator{
				itest.MissingFile{Path: "bbin/bb"},
				itest.HasContent{Path: "etc/somefile", Content: "foobar"},
				itest.HasContent{Path: somefile, Content: "foobar"},
				itest.HasContent{Path: "etc/somefile2", Content: "spongebob"},
				// TODO: This behavior is weird.
				itest.HasFile{Path: "uroot_test.go"},
				itest.HasFile{Path: filepath.Join(cwd, "uroot_test.go")},
				itest.HasDir{Path: "somedir"},
				itest.HasContent{Path: "somedir/somefile", Content: "foobar"},
			},
		},
		{
			name: "files conflict",
			opts: Opts{
				TempDir: dir,
				ExtraFiles: []string{
					somefile + ":etc/somefile",
					somefile2 + ":etc/somefile",
				},
			},
			errs: []error{os.ErrExist},
			validators: []itest.ArchiveValidator{
				itest.IsEmpty{},
			},
		},
		{
			name: "file does not exist",
			opts: Opts{
				TempDir: dir,
				ExtraFiles: []string{
					filepath.Join(dir, "doesnotexist") + ":etc/somefile",
				},
			},
			errs: []error{os.ErrNotExist},
			validators: []itest.ArchiveValidator{
				itest.IsEmpty{},
			},
		},
		/* TODO: case is broken.
		{
			name: "files invalid syntax 1",
			opts: Opts{
				TempDir: dir,
				ExtraFiles: []string{
					":etc/somefile",
				},
			},
			//errs: []error{os.ErrExist},
			validators: []itest.ArchiveValidator{
				itest.IsEmpty{},
			},
		},
		*/
		/* TODO: case is broken.
		{
			name: "files invalid syntax 2",
			opts: Opts{
				TempDir: dir,
				ExtraFiles: []string{
					somefile + ":",
				},
			},
			//errs: []error{os.ErrExist},
			validators: []itest.ArchiveValidator{
				itest.IsEmpty{},
			},
		},
		*/
		// TODO: files are directories.
		{
			name: "file conflicts with init",
			opts: Opts{
				TempDir: dir,
				InitCmd: "/bin/systemd",
				ExtraFiles: []string{
					somefile + ":init",
				},
			},
			errs: []error{os.ErrExist, errInitSymlink},
			validators: []itest.ArchiveValidator{
				itest.IsEmpty{},
			},
		},
		{
			name: "file conflicts with uinit flags",
			opts: Opts{
				TempDir:   dir,
				UinitArgs: []string{"-foo", "-bar"},
				ExtraFiles: []string{
					somefile + ":etc/uinit.flags",
				},
			},
			errs: []error{os.ErrExist, errUinitArgs},
			validators: []itest.ArchiveValidator{
				itest.IsEmpty{},
			},
		},
		{
			name: "file conflicts with uinit",
			opts: Opts{
				TempDir:  dir,
				UinitCmd: "/bin/systemd",
				ExtraFiles: []string{
					somefile + ":bin/uinit",
				},
			},
			errs: []error{os.ErrExist, errUinitSymlink},
			validators: []itest.ArchiveValidator{
				itest.IsEmpty{},
			},
		},
		{
			name: "file conflicts with sh",
			opts: Opts{
				TempDir:      dir,
				DefaultShell: "/bin/systemd",
				ExtraFiles: []string{
					somefile + ":bin/sh",
				},
			},
			errs: []error{os.ErrExist, errDefaultshSymlink},
			validators: []itest.ArchiveValidator{
				itest.IsEmpty{},
			},
		},
		{
			name: "file conflicts with defaultsh",
			opts: Opts{
				TempDir:      dir,
				DefaultShell: "/bin/systemd",
				ExtraFiles: []string{
					somefile + ":bin/defaultsh",
				},
			},
			errs: []error{os.ErrExist, errDefaultshSymlink},
			validators: []itest.ArchiveValidator{
				itest.IsEmpty{},
			},
		},
		{
			name: "file does not conflict if default files not specified",
			opts: Opts{
				TempDir: dir,
				// No DefaultShell, Init, or UinitCmd.
				ExtraFiles: []string{
					somefile + ":bin/defaultsh",
					somefile + ":bin/sh",
					somefile + ":bin/uinit",
					somefile + ":etc/uinit.flags",
					somefile + ":init",
				},
			},
			validators: []itest.ArchiveValidator{
				itest.HasContent{Path: "bin/defaultsh", Content: "foobar"},
				itest.HasContent{Path: "bin/sh", Content: "foobar"},
				itest.HasContent{Path: "bin/uinit", Content: "foobar"},
				itest.HasContent{Path: "etc/uinit.flags", Content: "foobar"},
				itest.HasContent{Path: "init", Content: "foobar"},
			},
		},
		{
			name: "init specified, but not in commands",
			opts: Opts{
				Env:          golang.Default(golang.DisableCGO()),
				TempDir:      dir,
				DefaultShell: "zoocar",
				InitCmd:      "foobar",
				UrootSource:  urootpath,
				Commands: []Commands{
					{
						Builder: builder.Binary,
						Packages: []string{
							"github.com/u-root/u-root/cmds/core/ls",
						},
					},
				},
			},
			errs: []error{errSymlink, errInitSymlink},
			validators: []itest.ArchiveValidator{
				itest.IsEmpty{},
			},
		},
		/* TODO: case broken.
		{
			name: "init not resolvable",
			opts: Opts{
				TempDir: dir,
				InitCmd: "init",
			},
			errs: []error{errSymlink, errInitSymlink},
			validators: []itest.ArchiveValidator{
				itest.IsEmpty{},
			},
		},
		*/
		{
			name: "init symlinked to absolute path",
			opts: Opts{
				TempDir: dir,
				InitCmd: "/bin/systemd",
			},
			validators: []itest.ArchiveValidator{
				itest.HasRecord{R: cpio.Symlink("init", "bin/systemd")},
			},
		},
		{
			name: "multi-mode archive",
			opts: Opts{
				Env:             golang.Default(golang.DisableCGO()),
				TempDir:         dir,
				ExtraFiles:      nil,
				UseExistingInit: false,
				InitCmd:         "init",
				DefaultShell:    "ls",
				UrootSource:     urootpath,
				Commands: []Commands{
					{
						Builder: builder.Busybox,
						Packages: []string{
							"github.com/u-root/u-root/cmds/core/init",
							"github.com/u-root/u-root/cmds/core/ls",
						},
					},
					{
						Builder: builder.Binary,
						Packages: []string{
							"github.com/u-root/u-root/cmds/core/cp",
							"github.com/u-root/u-root/cmds/core/dd",
						},
					},
				},
			},
			validators: []itest.ArchiveValidator{
				itest.HasRecord{R: cpio.Symlink("init", "bbin/init")},

				// bb mode.
				itest.HasFile{Path: "bbin/bb"},
				itest.HasRecord{R: cpio.Symlink("bbin/init", "bb")},
				itest.HasRecord{R: cpio.Symlink("bbin/ls", "bb")},
				itest.HasRecord{R: cpio.Symlink("bin/defaultsh", "../bbin/ls")},
				itest.HasRecord{R: cpio.Symlink("bin/sh", "../bbin/ls")},

				// binary mode.
				itest.HasFile{Path: "bin/cp"},
				itest.HasFile{Path: "bin/dd"},
			},
		},
		{
			name: "glob fail",
			opts: Opts{
				Env:         golang.Default(golang.DisableCGO()),
				TempDir:     dir,
				UrootSource: urootpath,
				Commands:    BinaryCmds("github.com/u-root/u-root/cmds/notexist/*"),
			},
			errs: []error{errResolvePackage},
			validators: []itest.ArchiveValidator{
				itest.IsEmpty{},
			},
		},
		{
			name: "tmp not writable",
			opts: Opts{
				Env:         golang.Default(golang.DisableCGO()),
				TempDir:     tmp400,
				UrootSource: urootpath,
				Commands:    BinaryCmds("github.com/u-root/u-root/cmds/core/..."),
			},
			errs: []error{os.ErrPermission},
			validators: []itest.ArchiveValidator{
				itest.IsEmpty{},
			},
		},
	} {
		t.Run(fmt.Sprintf("Test %d [%s]", i, tt.name), func(t *testing.T) {
			archive := inMemArchive{cpio.InMemArchive()}
			tt.opts.OutputFile = archive
			err := CreateInitramfs(l, tt.opts)
			for _, want := range tt.errs {
				if !errors.Is(err, want) {
					t.Errorf("CreateInitramfs = %v, want %v", err, want)
				}
			}
			if err != nil && len(tt.errs) == 0 {
				t.Errorf("CreateInitramfs = %v, want %v", err, nil)
			}

			for _, v := range tt.validators {
				if err := v.Validate(archive.Archive); err != nil {
					t.Errorf("validator failed: %v / archive:\n%s", err, archive)
				}
			}
		})
	}
}
