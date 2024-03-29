// Copyright 2022 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package builder

import (
	"errors"
	"fmt"
	"testing"

	"github.com/u-root/gobusybox/src/pkg/golang"
	"github.com/u-root/mkuimage/uimage/initramfs"
	"github.com/u-root/uio/llog"
)

func TestBinaryBuild(t *testing.T) {
	opts := Opts{
		Env: golang.Default(golang.DisableCGO()),
		Packages: []string{
			"../../cmd/mkuimage",
			"github.com/u-root/u-root/cmds/core/init",
			"cmd/test2json",
		},
		TempDir: t.TempDir(),
	}
	af := initramfs.NewFiles()
	var b BinaryBuilder
	if err := b.Build(llog.Test(t), af, opts); err != nil {
		t.Fatalf("Build(%v, %v); %v != nil", af, opts, err)
	}

	mustContain := []string{
		"bin/mkuimage",
		"bin/test2json",
		"bin/init",
	}
	for _, name := range mustContain {
		if !af.Contains(name) {
			t.Errorf("expected files to include %q; archive: %v", name, af)
		}
	}
}

func TestBinaryBuildError(t *testing.T) {
	for i, tt := range []struct {
		opts Opts
		want error
	}{
		{
			opts: Opts{
				Env: golang.Default(golang.DisableCGO()),
				Packages: []string{
					// Does not exist.
					"../../cmd/foobar",
				},
				TempDir:   t.TempDir(),
				BinaryDir: "bbin",
			},
			want: ErrBinaryFailed,
		},
		{
			opts: Opts{
				Env: golang.Default(golang.DisableCGO()),
				Packages: []string{
					"../../cmd/mkuimage",
				},
				BinaryDir: "bbin",
			},
			want: ErrTempDirMissing,
		},
		{
			opts: Opts{
				TempDir: t.TempDir(),
				Packages: []string{
					"../../cmd/mkuimage",
				},
				BinaryDir: "bbin",
			},
			want: ErrEnvMissing,
		},
	} {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			af := initramfs.NewFiles()
			var b BinaryBuilder
			if err := b.Build(llog.Test(t), af, tt.opts); !errors.Is(err, tt.want) {
				t.Errorf("Build = %v, want %v", err, tt.want)
			}
		})
	}
}
