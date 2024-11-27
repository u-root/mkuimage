// Copyright 2024 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package templates

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/u-root/gobusybox/src/pkg/golang"
	"github.com/u-root/mkuimage/uimage"
	"github.com/u-root/mkuimage/uimage/builder"
)

func TestMods(t *testing.T) {
	for _, tt := range []struct {
		name   string
		tpl    string
		config string
		base   []uimage.Modifier
		want   *uimage.Opts
		err    error
	}{
		{
			name: "ok",
			tpl: `
commands:
  core:
    - github.com/u-root/u-root/cmds/core/ip
    - github.com/u-root/u-root/cmds/core/init
    - github.com/u-root/u-root/cmds/core/gosh

  minimal:
    - github.com/u-root/u-root/cmds/core/ls
    - github.com/u-root/u-root/cmds/core/init

configs:
  plan9:
    goarch: amd64
    goos: plan9
    build_tags: [grpcnotrace]
    files:
      - /bin/bash
    init: init
    uinit: gosh script.sh
    shell: gosh
    commands:
      - builder: bb
        commands: [core, minimal]
      - builder: bb
        commands: [./u-bmc/cmd/foo]
      - builder: binary
        commands: [./u-bmc/cmd/bar]
      - builder: binary
        commands: [cmd/test2json]
`,
			config: "plan9",
			want: &uimage.Opts{
				Env:          golang.Default(golang.WithGOARCH("amd64"), golang.WithGOOS("plan9"), golang.WithBuildTag("grpcnotrace")),
				ExtraFiles:   []string{"/bin/bash"},
				InitCmd:      "init",
				UinitCmd:     "gosh",
				UinitArgs:    []string{"script.sh"},
				DefaultShell: "gosh",
				Commands: []uimage.Commands{
					{
						Builder: builder.Busybox,
						Packages: []string{
							"github.com/u-root/u-root/cmds/core/ip",
							"github.com/u-root/u-root/cmds/core/init",
							"github.com/u-root/u-root/cmds/core/gosh",
							"github.com/u-root/u-root/cmds/core/ls",
							"github.com/u-root/u-root/cmds/core/init",
							"./u-bmc/cmd/foo",
						},
					},
					{
						Builder:  builder.Binary,
						Packages: []string{"./u-bmc/cmd/bar"},
					},
					{
						Builder:  builder.Binary,
						Packages: []string{"cmd/test2json"},
					},
				},
			},
		},
		{
			name: "missing_config",
			tpl: `
configs:
  plan9:
    goarch: amd64
    goos: plan9
`,
			config: "plan10",
			err:    ErrTemplateNotExist,
		},
		{
			name: "no config",
			tpl: `
configs:
  plan9:
    goarch: amd64
    goos: plan9
`,
			config: "",
		},
		{
			name: "override base",
			tpl: `
configs:
  noinit:
    init: ""
`,
			config: "noinit",
			base: []uimage.Modifier{
				uimage.WithInit("init"),
			},
			want: &uimage.Opts{
				Env: golang.Default(),
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tpl, err := TemplateFrom([]byte(tt.tpl))
			if err != nil {
				t.Fatal(err)
			}
			mods, err := tpl.Uimage(tt.config)
			if !errors.Is(err, tt.err) {
				t.Fatalf("UimageMods = %v, want %v", err, tt.err)
			}
			if len(mods) == 0 {
				return
			}
			got, err := uimage.OptionsFor(append(tt.base, mods...)...)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Logf("got: %#v", got)
				t.Logf("want: %#v", tt.want)
				t.Errorf("not equal")
			}
		})
	}
}

func TestTemplateErr(t *testing.T) {
	if _, err := TemplateFrom([]byte("\t")); err == nil {
		t.Fatal("Expected error")
	}

	d := t.TempDir()
	wd, _ := os.Getwd()
	_ = os.Chdir(d)
	defer func() { _ = os.Chdir(wd) }()

	if _, err := Template(); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Template = %v, want ErrNotExist", err)
	}
	if _, err := TemplateFromFile(filepath.Join(d, "foobar")); !os.IsNotExist(err) {
		t.Fatalf("Template = %v, want not exist", err)
	}
}
