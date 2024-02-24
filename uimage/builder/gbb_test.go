// Copyright 2021 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package builder

import (
	"errors"
	"os"
	"testing"

	"github.com/u-root/gobusybox/src/pkg/golang"
	"github.com/u-root/mkuimage/cpio"
	"github.com/u-root/mkuimage/uimage/initramfs"
	"github.com/u-root/uio/llog"
)

func TestGBBBuild(t *testing.T) {
	dir := t.TempDir()

	opts := Opts{
		Env: golang.Default(golang.DisableCGO()),
		Packages: []string{
			"../../cmd/uimage",
		},
		TempDir: dir,
	}
	af := initramfs.NewFiles()
	var gbb GBBBuilder
	if err := gbb.Build(llog.Test(t), af, opts); err != nil {
		t.Fatalf("Build(%v, %v); %v != nil", af, opts, err)
	}

	mustContain := []string{
		"bbin/uimage",
		"bbin/bb",
	}
	for _, name := range mustContain {
		if !af.Contains(name) {
			t.Errorf("expected files to include %q; archive: %v", name, af)
		}
	}
}

func TestGBBBuildError(t *testing.T) {
	for _, tt := range []struct {
		gbb   GBBBuilder
		files []cpio.Record
		opts  Opts
		want  error
	}{
		{
			opts: Opts{
				Env: golang.Default(golang.DisableCGO()),
				Packages: []string{
					"../../cmd/uimage",
				},
				BinaryDir: "bbin",
			},
			want: ErrTempDirMissing,
		},
		{
			opts: Opts{
				TempDir: t.TempDir(),
				Packages: []string{
					"../../cmd/uimage",
				},
				BinaryDir: "bbin",
			},
			want: ErrEnvMissing,
		},
		{
			opts: Opts{
				Env:     golang.Default(golang.DisableCGO()),
				TempDir: t.TempDir(),
				Packages: []string{
					"../../cmd/uimage",
				},
				BinaryDir: "bbin",
			},
			files: []cpio.Record{
				cpio.StaticFile("bbin/bb", "", 0o777),
			},
			want: os.ErrExist,
		},
		{
			opts: Opts{
				Env:     golang.Default(golang.DisableCGO()),
				TempDir: t.TempDir(),
				Packages: []string{
					"../../cmd/uimage",
				},
				BinaryDir: "bbin",
			},
			files: []cpio.Record{
				cpio.StaticFile("bbin/uimage", "", 0o777),
			},
			want: os.ErrExist,
		},
		{
			opts: Opts{
				Env:     golang.Default(golang.DisableCGO()),
				TempDir: t.TempDir(),
				Packages: []string{
					"../../cmd/uimage",
				},
				BinaryDir: "bbin",
			},
			files: []cpio.Record{
				cpio.StaticFile("bbin/uimage", "", 0o777),
			},
			gbb:  GBBBuilder{ShellBang: true},
			want: os.ErrExist,
		},
	} {
		af := initramfs.NewFiles()
		for _, f := range tt.files {
			_ = af.AddRecord(f)
		}
		if err := tt.gbb.Build(llog.Test(t), af, tt.opts); !errors.Is(err, tt.want) {
			t.Errorf("Build = %v, want %v", err, tt.want)
		}
	}
}
