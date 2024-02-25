// Copyright 2018 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package uimage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"

	"github.com/u-root/gobusybox/src/pkg/golang"
	"github.com/u-root/mkuimage/cpio"
	"github.com/u-root/mkuimage/uimage/builder"
	"github.com/u-root/mkuimage/uimage/initramfs"
	itest "github.com/u-root/mkuimage/uimage/initramfs/test"
	"github.com/u-root/uio/llog"
)

func archive(tb testing.TB, r ...cpio.Record) *cpio.Archive {
	tb.Helper()
	a, err := cpio.ArchiveFromRecords(r)
	if err != nil {
		tb.Fatal(err)
	}
	return a
}

func TestCreateInitramfs(t *testing.T) {
	dir := t.TempDir()
	syscall.Umask(0)

	tmp777 := filepath.Join(dir, "tmp777")
	_ = os.MkdirAll(tmp777, 0o777)
	tmp400 := filepath.Join(dir, "tmp400")
	_ = os.MkdirAll(tmp400, 0o400)

	somedir := filepath.Join(dir, "dir")
	_ = os.MkdirAll(somedir, 0o777)
	somefile := filepath.Join(dir, "dir", "somefile")
	somefile2 := filepath.Join(dir, "dir", "somefile2")
	_ = os.WriteFile(somefile, []byte("foobar"), 0o777)
	_ = os.WriteFile(somefile2, []byte("spongebob"), 0o777)

	cwd, _ := os.Getwd()

	l := llog.Test(t)

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
					"uimage_test.go",
					filepath.Join(cwd, "uimage_test.go"),
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
				itest.HasFile{Path: "uimage_test.go"},
				itest.HasFile{Path: filepath.Join(cwd, "uimage_test.go")},
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
		{
			name: "files invalid syntax 1",
			opts: Opts{
				TempDir: dir,
				ExtraFiles: []string{
					":etc/somefile",
				},
			},
			errs: []error{os.ErrInvalid},
			validators: []itest.ArchiveValidator{
				itest.IsEmpty{},
			},
		},
		{
			name: "files invalid syntax 2",
			opts: Opts{
				TempDir: dir,
				ExtraFiles: []string{
					somefile + ":",
				},
			},
			errs: []error{os.ErrInvalid},
			validators: []itest.ArchiveValidator{
				itest.IsEmpty{},
			},
		},
		{
			name: "files are directories",
			opts: Opts{
				TempDir: dir,
				ExtraFiles: []string{
					somedir + ":etc/foo/bar",
				},
			},
			validators: []itest.ArchiveValidator{
				itest.HasDir{Path: "etc"},
				itest.HasDir{Path: "etc/foo"},
				itest.HasDir{Path: "etc/foo/bar"},
				itest.HasContent{Path: "etc/foo/bar/somefile", Content: "foobar"},
				itest.HasContent{Path: "etc/foo/bar/somefile2", Content: "spongebob"},
			},
		},
		{
			name: "files are directories SkipLDD",
			opts: Opts{
				TempDir: dir,
				ExtraFiles: []string{
					somedir + ":etc/foo/bar",
				},
				SkipLDD: true,
			},
			validators: []itest.ArchiveValidator{
				itest.HasDir{Path: "etc"},
				itest.HasDir{Path: "etc/foo"},
				itest.HasDir{Path: "etc/foo/bar"},
				itest.HasContent{Path: "etc/foo/bar/somefile", Content: "foobar"},
				itest.HasContent{Path: "etc/foo/bar/somefile2", Content: "spongebob"},
			},
		},
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
							"github.com/u-root/u-root/cmds/core/echo",
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
				itest.HasFile{Path: "bin/echo"},
			},
		},
		{
			name: "glob fail",
			opts: Opts{
				Env:      golang.Default(golang.DisableCGO()),
				TempDir:  dir,
				Commands: BinaryCmds("github.com/u-root/u-root/cmds/notexist/*"),
			},
			errs: []error{errResolvePackage},
			validators: []itest.ArchiveValidator{
				itest.IsEmpty{},
			},
		},
		{
			name: "tmp not writable",
			opts: Opts{
				Env:      golang.Default(golang.DisableCGO()),
				TempDir:  tmp400,
				Commands: BinaryCmds("github.com/u-root/u-root/cmds/core/..."),
			},
			errs: []error{os.ErrPermission},
			validators: []itest.ArchiveValidator{
				itest.IsEmpty{},
			},
		},
		{
			name: "cpio no path given",
			opts: Opts{
				TempDir:    dir,
				InitCmd:    "/bin/systemd",
				OutputFile: &initramfs.CPIOFile{},
			},
			errs: []error{initramfs.ErrNoPath},
		},
		{
			name: "dir no path given",
			opts: Opts{
				TempDir:    dir,
				InitCmd:    "/bin/systemd",
				OutputFile: &initramfs.Dir{},
			},
			errs: []error{initramfs.ErrNoPath},
		},
		{
			name: "dir failed to create",
			opts: Opts{
				TempDir:    dir,
				InitCmd:    "/bin/systemd",
				OutputFile: &initramfs.Dir{Path: filepath.Join(tmp400, "foobar")},
			},
			errs: []error{os.ErrPermission},
		},
		{
			name: "cpio failed to create",
			opts: Opts{
				TempDir:    dir,
				InitCmd:    "/bin/systemd",
				OutputFile: &initramfs.CPIOFile{Path: filepath.Join(tmp400, "foobar")},
			},
			errs: []error{os.ErrPermission},
		},
		{
			name: "cpio basefile no path given",
			opts: Opts{
				TempDir:     dir,
				InitCmd:     "/bin/systemd",
				BaseArchive: &initramfs.CPIOFile{},
			},
			errs: []error{initramfs.ErrNoPath},
		},
		{
			name: "symlinks",
			opts: Opts{
				Env:          golang.Default(golang.DisableCGO()),
				TempDir:      dir,
				InitCmd:      "init",
				DefaultShell: "ls",
				Symlinks: map[string]string{
					"ubin/foo":  "ls",
					"ubin/fooa": "/bin/systemd",
				},
				Commands: BusyboxCmds(
					"github.com/u-root/u-root/cmds/core/init",
					"github.com/u-root/u-root/cmds/core/ls",
				),
			},
			validators: []itest.ArchiveValidator{
				itest.HasFile{Path: "bbin/bb"},
				itest.HasRecord{R: cpio.Symlink("bbin/init", "bb")},
				itest.HasRecord{R: cpio.Symlink("bbin/ls", "bb")},
				itest.HasRecord{R: cpio.Symlink("bin/defaultsh", "../bbin/ls")},
				itest.HasRecord{R: cpio.Symlink("bin/sh", "../bbin/ls")},
				itest.HasRecord{R: cpio.Symlink("ubin/foo", "../bbin/ls")},
				itest.HasRecord{R: cpio.Symlink("ubin/fooa", "../bin/systemd")},
			},
		},
		{
			name: "dup symlinks",
			opts: Opts{
				Env:          golang.Default(golang.DisableCGO()),
				TempDir:      dir,
				InitCmd:      "init",
				DefaultShell: "ls",
				Symlinks: map[string]string{
					"/bbin/ls": "init",
				},
				Commands: BusyboxCmds(
					"github.com/u-root/u-root/cmds/core/init",
					"github.com/u-root/u-root/cmds/core/ls",
				),
			},
			errs: []error{os.ErrExist},
			validators: []itest.ArchiveValidator{
				itest.IsEmpty{},
			},
		},
	} {
		t.Run(fmt.Sprintf("Test %d [%s]", i, tt.name), func(t *testing.T) {
			archive := cpio.InMemArchive()
			if tt.opts.OutputFile == nil {
				tt.opts.OutputFile = &initramfs.Archive{Archive: archive}
			}
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
				if err := v.Validate(archive); err != nil {
					t.Errorf("validator failed: %v / archive:\n%s", err, archive)
				}
			}
		})
	}
}

func TestCreateInitramfsWithAPI(t *testing.T) {
	dir := t.TempDir()
	syscall.Umask(0)

	tmp777 := filepath.Join(dir, "tmp777")
	_ = os.MkdirAll(tmp777, 0o777)
	tmp400 := filepath.Join(dir, "tmp400")
	_ = os.MkdirAll(tmp400, 0o400)

	somedir := filepath.Join(dir, "dir")
	_ = os.MkdirAll(somedir, 0o777)
	somefile := filepath.Join(dir, "dir", "somefile")
	somefile2 := filepath.Join(dir, "dir", "somefile2")
	_ = os.WriteFile(somefile, []byte("foobar"), 0o777)
	_ = os.WriteFile(somefile2, []byte("spongebob"), 0o777)

	cwd, _ := os.Getwd()

	l := llog.Test(t)

	for i, tt := range []struct {
		name       string
		opts       []Modifier
		noOutput   bool
		errs       []error
		validators []itest.ArchiveValidator
	}{
		{
			name: "BB archive",
			opts: []Modifier{
				WithEnv(golang.DisableCGO()),
				WithTempDir(dir),
				WithInit("init"),
				WithShell("ls"),
				WithBusyboxCommands(
					"github.com/u-root/u-root/cmds/core/init",
					"github.com/u-root/u-root/cmds/core/ls",
				),
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
			opts: []Modifier{
				WithInit("init"),
			},
			errs: []error{os.ErrNotExist},
			validators: []itest.ArchiveValidator{
				itest.IsEmpty{},
			},
		},
		{
			name: "no commands",
			opts: []Modifier{
				WithTempDir(dir),
			},
			validators: []itest.ArchiveValidator{
				itest.MissingFile{Path: "bbin/bb"},
			},
		},
		{
			name: "files",
			opts: []Modifier{
				WithTempDir(dir),
				WithFiles(
					somefile+":etc/somefile",
					somefile2+":etc/somefile2",
					somefile,
					// Empty is ignored.
					"",
					"uimage_test.go",
					filepath.Join(cwd, "uimage_test.go"),
					// Parent directory is created.
					somefile+":somedir/somefile",
				),
			},
			validators: []itest.ArchiveValidator{
				itest.MissingFile{Path: "bbin/bb"},
				itest.HasContent{Path: "etc/somefile", Content: "foobar"},
				itest.HasContent{Path: somefile, Content: "foobar"},
				itest.HasContent{Path: "etc/somefile2", Content: "spongebob"},
				// TODO: This behavior is weird.
				itest.HasFile{Path: "uimage_test.go"},
				itest.HasFile{Path: filepath.Join(cwd, "uimage_test.go")},
				itest.HasDir{Path: "somedir"},
				itest.HasContent{Path: "somedir/somefile", Content: "foobar"},
			},
		},
		{
			name: "files conflict",
			opts: []Modifier{
				WithTempDir(dir),
				WithFiles(
					somefile+":etc/somefile",
					somefile2+":etc/somefile",
				),
			},
			errs: []error{os.ErrExist},
			validators: []itest.ArchiveValidator{
				itest.IsEmpty{},
			},
		},
		{
			name: "file does not exist",
			opts: []Modifier{
				WithTempDir(dir),
				WithFiles(filepath.Join(dir, "doesnotexist") + ":etc/somefile"),
			},
			errs: []error{os.ErrNotExist},
			validators: []itest.ArchiveValidator{
				itest.IsEmpty{},
			},
		},
		{
			name: "files invalid syntax 1",
			opts: []Modifier{
				WithTempDir(dir),
				WithFiles(":etc/somefile"),
			},
			errs: []error{os.ErrInvalid},
			validators: []itest.ArchiveValidator{
				itest.IsEmpty{},
			},
		},
		{
			name: "files invalid syntax 2",
			opts: []Modifier{
				WithTempDir(dir),
				WithFiles(somefile + ":"),
			},
			errs: []error{os.ErrInvalid},
			validators: []itest.ArchiveValidator{
				itest.IsEmpty{},
			},
		},
		{
			name: "files are directories",
			opts: []Modifier{
				WithTempDir(dir),
				WithFiles(somedir + ":etc/foo/bar"),
			},
			validators: []itest.ArchiveValidator{
				itest.HasDir{Path: "etc"},
				itest.HasDir{Path: "etc/foo"},
				itest.HasDir{Path: "etc/foo/bar"},
				itest.HasContent{Path: "etc/foo/bar/somefile", Content: "foobar"},
				itest.HasContent{Path: "etc/foo/bar/somefile2", Content: "spongebob"},
			},
		},
		{
			name: "files are directories SkipLDD",
			opts: []Modifier{
				WithTempDir(dir),
				WithFiles(somedir + ":etc/foo/bar"),
				WithSkipLDD(),
			},
			validators: []itest.ArchiveValidator{
				itest.HasDir{Path: "etc"},
				itest.HasDir{Path: "etc/foo"},
				itest.HasDir{Path: "etc/foo/bar"},
				itest.HasContent{Path: "etc/foo/bar/somefile", Content: "foobar"},
				itest.HasContent{Path: "etc/foo/bar/somefile2", Content: "spongebob"},
			},
		},
		{
			name: "file conflicts with init",
			opts: []Modifier{
				WithTempDir(dir),
				WithInit("/bin/systemd"),
				WithFiles(somefile + ":init"),
			},
			errs: []error{os.ErrExist, errInitSymlink},
			validators: []itest.ArchiveValidator{
				itest.IsEmpty{},
			},
		},
		{
			name: "file conflicts with uinit flags",
			opts: []Modifier{
				WithTempDir(dir),
				WithUinitCommand("huh -foo -bar"),
				WithFiles(somefile + ":etc/uinit.flags"),
			},
			errs: []error{os.ErrExist, errUinitArgs},
			validators: []itest.ArchiveValidator{
				itest.IsEmpty{},
			},
		},
		{
			name: "file conflicts with uinit",
			opts: []Modifier{
				WithTempDir(dir),
				WithUinit("/bin/systemd"),
				WithFiles(somefile + ":bin/uinit"),
			},
			errs: []error{os.ErrExist, errUinitSymlink},
			validators: []itest.ArchiveValidator{
				itest.IsEmpty{},
			},
		},
		{
			name: "file conflicts with sh",
			opts: []Modifier{
				WithTempDir(dir),
				WithShell("/bin/systemd"),
				WithFiles(somefile + ":bin/sh"),
			},
			errs: []error{os.ErrExist, errDefaultshSymlink},
			validators: []itest.ArchiveValidator{
				itest.IsEmpty{},
			},
		},
		{
			name: "file conflicts with defaultsh",
			opts: []Modifier{
				WithTempDir(dir),
				WithShell("/bin/systemd"),
				WithFiles(somefile + ":bin/defaultsh"),
			},
			errs: []error{os.ErrExist, errDefaultshSymlink},
			validators: []itest.ArchiveValidator{
				itest.IsEmpty{},
			},
		},
		{
			name: "file does not conflict if default files not specified",
			opts: []Modifier{
				WithTempDir(dir),
				// No DefaultShell, Init, or UinitCmd.
				WithFiles(
					somefile+":bin/defaultsh",
					somefile+":bin/sh",
					somefile+":bin/uinit",
					somefile+":etc/uinit.flags",
					somefile+":init",
				),
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
			opts: []Modifier{
				WithTempDir(dir),
				WithEnv(golang.DisableCGO()),
				WithInit("foobar"),
				WithBinaryCommands(
					"github.com/u-root/u-root/cmds/core/ls",
				),
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
			opts: []Modifier{
				WithTempDir(dir),
				WithInit("/bin/systemd"),
			},
			validators: []itest.ArchiveValidator{
				itest.HasRecord{R: cpio.Symlink("init", "bin/systemd")},
			},
		},
		{
			name: "multi-mode archive",
			opts: []Modifier{
				WithTempDir(dir),
				WithEnv(golang.DisableCGO()),
				WithInit("init"),
				WithShell("ls"),
				WithBusyboxCommands(
					"github.com/u-root/u-root/cmds/core/init",
					"github.com/u-root/u-root/cmds/core/ls",
				),
				WithBinaryCommands(
					"github.com/u-root/u-root/cmds/core/cp",
					"github.com/u-root/u-root/cmds/core/echo",
				),
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
				itest.HasFile{Path: "bin/echo"},
			},
		},
		{
			name: "glob fail",
			opts: []Modifier{
				WithTempDir(dir),
				WithEnv(golang.DisableCGO()),
				WithBinaryCommands("github.com/u-root/u-root/cmds/notexist/*"),
			},
			errs: []error{errResolvePackage},
			validators: []itest.ArchiveValidator{
				itest.IsEmpty{},
			},
		},
		{
			name: "tmp not writable",
			opts: []Modifier{
				WithEnv(golang.DisableCGO()),
				WithTempDir(tmp400),
				WithBinaryCommands("github.com/u-root/u-root/cmds/core/..."),
			},
			errs: []error{os.ErrPermission},
			validators: []itest.ArchiveValidator{
				itest.IsEmpty{},
			},
		},
		{
			name: "cpio no path given",
			opts: []Modifier{
				WithTempDir(dir),
				WithInit("/bin/systemd"),
				WithOutput(&initramfs.CPIOFile{}),
			},
			noOutput: true,
			errs:     []error{initramfs.ErrNoPath},
		},
		{
			name: "dir no path given",
			opts: []Modifier{
				WithTempDir(dir),
				WithInit("/bin/systemd"),
				WithOutput(&initramfs.Dir{}),
			},
			noOutput: true,
			errs:     []error{initramfs.ErrNoPath},
		},
		{
			name: "dir failed to create",
			opts: []Modifier{
				WithTempDir(dir),
				WithInit("/bin/systemd"),
				WithOutput(&initramfs.Dir{Path: filepath.Join(tmp400, "foobar")}),
			},
			noOutput: true,
			errs:     []error{os.ErrPermission},
		},
		{
			name: "cpio failed to create",
			opts: []Modifier{
				WithTempDir(dir),
				WithInit("/bin/systemd"),
				WithOutput(&initramfs.CPIOFile{Path: filepath.Join(tmp400, "foobar")}),
			},
			noOutput: true,
			errs:     []error{os.ErrPermission},
		},
		{
			name: "cpio basefile no path given",
			opts: []Modifier{
				WithTempDir(dir),
				WithInit("/bin/systemd"),
				WithOutput(&initramfs.CPIOFile{}),
			},
			noOutput: true,
			errs:     []error{initramfs.ErrNoPath},
		},
		{
			name: "base archive",
			opts: []Modifier{
				WithTempDir(dir),
				WithInit("/bin/systemd"),
				WithBaseArchive(archive(t,
					cpio.StaticFile("etc/foo", "bar", 0o777),
				)),
			},
			validators: []itest.ArchiveValidator{
				itest.HasRecord{R: cpio.Symlink("init", "bin/systemd")},
				itest.HasContent{Path: "etc/foo", Content: "bar"},
			},
		},
		{
			name: "symlinks",
			opts: []Modifier{
				WithTempDir(dir),
				WithEnv(golang.DisableCGO()),
				WithInit("init"),
				WithShell("ls"),
				WithBusyboxCommands(
					"github.com/u-root/u-root/cmds/core/init",
					"github.com/u-root/u-root/cmds/core/ls",
				),
				WithSymlink("ubin/foo", "ls"),
				WithSymlink("ubin/fooa", "/bin/systemd"),
			},
			validators: []itest.ArchiveValidator{
				itest.HasFile{Path: "bbin/bb"},
				itest.HasRecord{R: cpio.Symlink("bbin/init", "bb")},
				itest.HasRecord{R: cpio.Symlink("bbin/ls", "bb")},
				itest.HasRecord{R: cpio.Symlink("bin/defaultsh", "../bbin/ls")},
				itest.HasRecord{R: cpio.Symlink("bin/sh", "../bbin/ls")},
				itest.HasRecord{R: cpio.Symlink("ubin/foo", "../bbin/ls")},
				itest.HasRecord{R: cpio.Symlink("ubin/fooa", "../bin/systemd")},
			},
		},
		{
			name: "dup symlinks",
			opts: []Modifier{
				WithTempDir(dir),
				WithEnv(golang.DisableCGO()),
				WithInit("init"),
				WithShell("ls"),
				WithBusyboxCommands(
					"github.com/u-root/u-root/cmds/core/init",
					"github.com/u-root/u-root/cmds/core/ls",
				),
				WithSymlink("/bbin/ls", "init"),
			},
			errs: []error{os.ErrExist},
			validators: []itest.ArchiveValidator{
				itest.IsEmpty{},
			},
		},
		{
			name: "dup with symlinks",
			opts: []Modifier{
				WithTempDir(dir),
				WithSymlink("ubin/ls", "/bin/foo"),
				WithSymlink("ubin/ls", "../bbin/ls"),
			},
			errs: []error{os.ErrExist},
			validators: []itest.ArchiveValidator{
				itest.IsEmpty{},
			},
		},
		{
			name: "shellbang",
			opts: []Modifier{
				WithTempDir(dir),
				WithEnv(golang.DisableCGO()),
				WithShellBang(true),
				WithBusyboxCommands(
					"github.com/u-root/u-root/cmds/core/init",
					"github.com/u-root/u-root/cmds/core/ls",
				),
			},
			validators: []itest.ArchiveValidator{
				itest.HasFile{Path: "bbin/bb"},
				itest.HasRecord{R: cpio.StaticFile("bbin/init", "#!/bbin/bb #!init\n", 0o755)},
				itest.HasRecord{R: cpio.StaticFile("bbin/ls", "#!/bbin/bb #!ls\n", 0o755)},
			},
		},
		{
			name: "shellbang after placement",
			opts: []Modifier{
				WithTempDir(dir),
				WithEnv(golang.DisableCGO()),
				WithBusyboxCommands(
					"github.com/u-root/u-root/cmds/core/init",
					"github.com/u-root/u-root/cmds/core/ls",
				),
				// Putting this after WithBusyboxCommands should not change the outcome.
				WithShellBang(true),
			},
			validators: []itest.ArchiveValidator{
				itest.HasFile{Path: "bbin/bb"},
				itest.HasRecord{R: cpio.StaticFile("bbin/init", "#!/bbin/bb #!init\n", 0o755)},
				itest.HasRecord{R: cpio.StaticFile("bbin/ls", "#!/bbin/bb #!ls\n", 0o755)},
			},
		},
		{
			name: "shellbang no busybox",
			opts: []Modifier{
				WithTempDir(dir),
				WithEnv(golang.DisableCGO()),
				WithBinaryCommands(
					"github.com/u-root/u-root/cmds/core/init",
				),
				// Putting this after WithBusyboxCommands should not change the outcome.
				WithShellBang(true),
			},
			validators: []itest.ArchiveValidator{
				itest.HasFile{Path: "bin/init"},
			},
		},
	} {
		t.Run(fmt.Sprintf("Test %d [%s]", i, tt.name), func(t *testing.T) {
			archive := cpio.InMemArchive()
			if !tt.noOutput {
				tt.opts = append(tt.opts, WithOutput(&initramfs.Archive{Archive: archive}))
			}
			err := Create(l, tt.opts...)
			for _, want := range tt.errs {
				if !errors.Is(err, want) {
					t.Errorf("CreateInitramfs = %v, want %v", err, want)
				}
			}
			if err != nil && len(tt.errs) == 0 {
				t.Errorf("CreateInitramfs = %v, want %v", err, nil)
			}

			for _, v := range tt.validators {
				if err := v.Validate(archive); err != nil {
					t.Errorf("validator failed: %v / archive:\n%s", err, archive)
				}
			}
		})
	}
}

func TestOptionsFor(t *testing.T) {
	for _, tt := range []struct {
		name string
		mods []Modifier
		want *Opts
	}{
		{
			name: "buildopts after",
			mods: []Modifier{
				WithBusyboxCommands(
					"github.com/u-root/u-root/cmds/core/init",
					"github.com/u-root/u-root/cmds/core/ls",
				),
				WithBusyboxBuildOpts(&golang.BuildOpts{NoStrip: true}),
			},
			want: &Opts{
				Env: golang.Default(),
				Commands: []Commands{
					{
						Builder: builder.Busybox,
						Packages: []string{
							"github.com/u-root/u-root/cmds/core/init",
							"github.com/u-root/u-root/cmds/core/ls",
						},
						BuildOpts: &golang.BuildOpts{NoStrip: true},
					},
				},
			},
		},
		{
			name: "buildopts before",
			mods: []Modifier{
				WithBusyboxBuildOpts(&golang.BuildOpts{NoStrip: true}),
				WithBusyboxCommands(
					"github.com/u-root/u-root/cmds/core/init",
					"github.com/u-root/u-root/cmds/core/ls",
				),
			},
			want: &Opts{
				Env: golang.Default(),
				Commands: []Commands{
					{
						Builder: builder.Busybox,
						Packages: []string{
							"github.com/u-root/u-root/cmds/core/init",
							"github.com/u-root/u-root/cmds/core/ls",
						},
						BuildOpts: &golang.BuildOpts{NoStrip: true},
					},
				},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := OptionsFor(tt.mods...)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("OptionsFor = \n%#v, want\n%#v", got, tt.want)
			}
		})
	}
}
