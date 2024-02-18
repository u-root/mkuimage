// Copyright 2024 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mkuimage

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/u-root/gobusybox/src/pkg/golang"
	"github.com/u-root/mkuimage/uimage"
	"github.com/u-root/mkuimage/uimage/builder"
	"github.com/u-root/mkuimage/uimage/initramfs"
	"github.com/u-root/mkuimage/uimage/templates"
	"github.com/u-root/uio/llog"
)

func TestOpts(t *testing.T) {
	for _, tt := range []struct {
		name string
		m    []uimage.Modifier
		tpl  *templates.Templates
		f    *Flags
		conf string
		cmds []string
		opts *uimage.Opts
		err  error
	}{
		{
			name: "cmdline only",
			m: []uimage.Modifier{
				uimage.WithReplaceEnv(golang.Default(golang.DisableCGO())),
				uimage.WithCPIOOutput("/tmp/initramfs.cpio"),
				uimage.WithTempDir("foo"),
			},
			f: &Flags{
				Commands:      CommandFlags{Builder: "bb", Mod: "readonly"},
				ArchiveFormat: "cpio",
				Init:          "init",
				Uinit:         "gosh script.sh",
				OutputFile:    "./foo.cpio",
				Files:         []string{"/bin/bash"},
			},
			cmds: []string{
				"github.com/u-root/u-root/cmds/core/init",
				"github.com/u-root/u-root/cmds/core/gosh",
			},
			opts: &uimage.Opts{
				Env:        golang.Default(golang.DisableCGO()),
				InitCmd:    "init",
				UinitCmd:   "gosh",
				UinitArgs:  []string{"script.sh"},
				OutputFile: &initramfs.CPIOFile{Path: "./foo.cpio"},
				ExtraFiles: []string{"/bin/bash"},
				TempDir:    "foo",
				Commands: []uimage.Commands{
					{
						Builder: &builder.GBBBuilder{},
						Packages: []string{
							"github.com/u-root/u-root/cmds/core/init",
							"github.com/u-root/u-root/cmds/core/gosh",
						},
					},
				},
			},
		},
		{
			name: "template and cmdline combo",
			m: []uimage.Modifier{
				uimage.WithReplaceEnv(golang.Default(golang.DisableCGO())),
				uimage.WithCPIOOutput("/tmp/initramfs.cpio"),
				uimage.WithTempDir("foo"),
			},
			tpl: &templates.Templates{
				Configs: map[string]templates.Config{
					"plan9": templates.Config{
						GOOS:      "plan9",
						GOARCH:    "amd64",
						BuildTags: []string{"grpcnotrace"},
						Uinit:     "gosh script.sh",
						Files:     []string{"foobar"},
						Commands: []templates.Command{
							{
								Builder: "bb",
								Commands: []string{
									"github.com/u-root/u-root/cmds/core/gosh",
								},
							},
							{
								Builder: "binary",
								Commands: []string{
									"cmd/test2json",
								},
							},
						},
					},
				},
			},
			conf: "plan9",
			f: &Flags{
				Commands:      CommandFlags{Builder: "bb", Mod: "readonly"},
				ArchiveFormat: "cpio",
				Init:          "init",
				Uinit:         "cat",
				OutputFile:    "./foo.cpio",
				Files:         []string{"/bin/bash"},
			},
			cmds: []string{
				"github.com/u-root/u-root/cmds/core/init",
				"github.com/u-root/u-root/cmds/core/cat",
			},
			opts: &uimage.Opts{
				Env:        golang.Default(golang.DisableCGO(), golang.WithGOOS("plan9"), golang.WithGOARCH("amd64"), golang.WithBuildTag("grpcnotrace")),
				InitCmd:    "init",
				UinitCmd:   "cat",
				OutputFile: &initramfs.CPIOFile{Path: "./foo.cpio"},
				ExtraFiles: []string{"foobar", "/bin/bash"},
				TempDir:    "foo",
				Commands: []uimage.Commands{
					{
						Builder: &builder.GBBBuilder{},
						Packages: []string{
							"github.com/u-root/u-root/cmds/core/gosh",
							"github.com/u-root/u-root/cmds/core/init",
							"github.com/u-root/u-root/cmds/core/cat",
						},
					},
					{
						Builder: builder.Binary,
						Packages: []string{
							"cmd/test2json",
						},
					},
				},
			},
		},
		{
			name: "expand cmdline config",
			m: []uimage.Modifier{
				uimage.WithReplaceEnv(golang.Default(golang.DisableCGO())),
				uimage.WithCPIOOutput("/tmp/initramfs.cpio"),
				uimage.WithTempDir("foo"),
			},
			f: &Flags{
				Commands:      CommandFlags{Builder: "bb", Mod: "readonly"},
				ArchiveFormat: "cpio",
				OutputFile:    "./foo.cpio",
				Files:         []string{"/bin/bash"},
			},
			tpl: &templates.Templates{
				Commands: map[string][]string{
					"core": []string{
						"github.com/u-root/u-root/cmds/core/init",
						"github.com/u-root/u-root/cmds/core/gosh",
					},
				},
			},
			cmds: []string{"core", "github.com/u-root/u-root/cmds/core/cat"},
			opts: &uimage.Opts{
				Env:        golang.Default(golang.DisableCGO()),
				OutputFile: &initramfs.CPIOFile{Path: "./foo.cpio"},
				ExtraFiles: []string{"/bin/bash"},
				TempDir:    "foo",
				Commands: []uimage.Commands{
					{
						Builder: &builder.GBBBuilder{},
						Packages: []string{
							"github.com/u-root/u-root/cmds/core/init",
							"github.com/u-root/u-root/cmds/core/gosh",
							"github.com/u-root/u-root/cmds/core/cat",
						},
					},
				},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			opts, err := uimageOpts(llog.Test(t), tt.m, tt.tpl, tt.f, tt.conf, tt.cmds)
			if !errors.Is(err, tt.err) {
				t.Errorf("opts = %v, want %v", err, tt.err)
			}
			if diff := cmp.Diff(opts, tt.opts, cmpopts.IgnoreFields(uimage.Opts{}, "BaseArchive")); diff != "" {
				t.Errorf("opts (-got, +want) = %v", diff)
			}
		})
	}
}
