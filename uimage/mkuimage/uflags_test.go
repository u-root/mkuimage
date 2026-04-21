// Copyright 2024 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mkuimage

import (
	"errors"
	"flag"
	"os"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/u-root/gobusybox/src/pkg/golang"
	"github.com/u-root/mkuimage/uimage"
	"github.com/u-root/mkuimage/uimage/builder"
)

func TestFlagErrors(t *testing.T) {
	for _, tt := range []struct {
		input []string
		err   error
	}{
		{
			input: []string{"-build=else", "-format=cpio"},
			err:   os.ErrInvalid,
		},
		{
			input: []string{"-format=else", "-build=bb"},
			err:   os.ErrInvalid,
		},
	} {
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		f := &Flags{}
		f.RegisterFlags(fs)
		if err := fs.Parse(tt.input); err != nil {
			t.Fatal(err)
		}
		if _, err := f.Modifiers(); !errors.Is(err, tt.err) {
			t.Errorf("Modifiers = %v, want %v", err, tt.err)
		}
	}
}

func TestFlags(t *testing.T) {
	for _, tt := range []struct {
		input []string
		want  *Flags
	}{
		{
			input: []string{"-build=bb", "-initcmd=foo"},
			want: &Flags{
				Init: String("foo"),
				Commands: CommandFlags{
					Builder:   "bb",
					BuildOpts: &golang.BuildOpts{},
				},
			},
		},
		{
			input: []string{"-build=bb"},
			want: &Flags{
				Commands: CommandFlags{
					Builder:   "bb",
					BuildOpts: &golang.BuildOpts{},
				},
			},
		},
		{
			input: []string{"-build=bb", "-initcmd=foo", "-uinitcmd=foo bar", "-tmp-dir=bla", "-defaultsh=gosh"},
			want: &Flags{
				Init:    String("foo"),
				Uinit:   String("foo bar"),
				TempDir: String("bla"),
				Shell:   String("gosh"),
				Commands: CommandFlags{
					Builder:   "bb",
					BuildOpts: &golang.BuildOpts{},
				},
			},
		},
		{
			input: []string{"-build=bb", "-initcmd=", "-uinitcmd=", "-tmp-dir=", "-defaultsh="},
			want: &Flags{
				Init:    String(""),
				Uinit:   String(""),
				TempDir: String(""),
				Shell:   String(""),
				Commands: CommandFlags{
					Builder:   "bb",
					BuildOpts: &golang.BuildOpts{},
				},
			},
		},
		{
			input: []string{"-build=bb", "-skip-ldd"},
			want: &Flags{
				SkipLDD: true,
				Commands: CommandFlags{
					Builder:   "bb",
					BuildOpts: &golang.BuildOpts{},
				},
			},
		},
		{
			input: []string{"-build=bb", "-skip-ldd=false"},
			want: &Flags{
				SkipLDD: false,
				Commands: CommandFlags{
					Builder:   "bb",
					BuildOpts: &golang.BuildOpts{},
				},
			},
		},
	} {
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		f := &Flags{}
		f.RegisterFlags(fs)
		if err := fs.Parse(tt.input); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(f, tt.want) {
			t.Errorf("Parse = %+v, want %+v", f, tt.want)
		}
	}
}

func TestFlagModifiers(t *testing.T) {
	for _, tt := range []struct {
		input []string
		base  []uimage.Modifier
		want  *uimage.Opts
		cmds  []string
	}{
		{
			// Override modifier defaults with empty.
			input: []string{"-build=bb", "-format=cpio", "-initcmd=", "-uinitcmd=", "-defaultsh=", "-tmp-dir="},
			base: []uimage.Modifier{
				uimage.WithInit("foo"),
				uimage.WithUinit("foo bar"),
				uimage.WithTempDir("foo"),
				uimage.WithShell("foo"),
			},
			cmds: []string{"foo"},
			want: &uimage.Opts{
				Env: golang.Default(),
				Commands: []uimage.Commands{
					{
						Builder:   &builder.GBBBuilder{},
						BuildOpts: &golang.BuildOpts{},
					},
				},
			},
		},
		{
			// Test that -skip-ldd flag sets SkipLDD to true
			input: []string{"-build=bb", "-format=cpio", "-skip-ldd"},
			base:  []uimage.Modifier{},
			cmds:  []string{"foo"},
			want: &uimage.Opts{
				Env:     golang.Default(),
				SkipLDD: true,
				Commands: []uimage.Commands{
					{
						Builder:   &builder.GBBBuilder{},
						BuildOpts: &golang.BuildOpts{},
					},
				},
			},
		},
		{
			// Test that without -skip-ldd flag, SkipLDD remains false
			input: []string{"-build=bb", "-format=cpio"},
			base:  []uimage.Modifier{},
			cmds:  []string{"foo"},
			want: &uimage.Opts{
				Env:     golang.Default(),
				SkipLDD: false,
				Commands: []uimage.Commands{
					{
						Builder:   &builder.GBBBuilder{},
						BuildOpts: &golang.BuildOpts{},
					},
				},
			},
		},
	} {
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		f := &Flags{}
		f.RegisterFlags(fs)
		if err := fs.Parse(tt.input); err != nil {
			t.Fatal(err)
		}
		mods, err := f.Modifiers()
		if err != nil {
			t.Errorf("Modifiers = %v", err)
		}
		opts, err := uimage.OptionsFor(append(tt.base, mods...)...)
		if err != nil {
			t.Errorf("Options = %v", err)
		}
		if diff := cmp.Diff(opts, tt.want, cmpopts.IgnoreFields(uimage.Opts{}, "BaseArchive")); diff != "" {
			t.Errorf("opts (-got, +want) = %v", diff)
		}
	}
}
