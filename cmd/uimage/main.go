// Copyright 2015-2024 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Command uimage builds CPIO archives with the given files and Go commands.
package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"slices"

	"github.com/u-root/gobusybox/src/pkg/bb/findpkg"
	"github.com/u-root/gobusybox/src/pkg/golang"
	"github.com/u-root/mkuimage/uimage"
	"github.com/u-root/mkuimage/uimage/mkuimage"
	"github.com/u-root/uio/cli"
	"github.com/u-root/uio/llog"
	"golang.org/x/exp/maps"
)

func main() {
	log.SetFlags(log.Ltime)
	l := llog.Default()

	env := golang.Default(golang.DisableCGO())
	f := &mkuimage.Flags{
		Commands:      mkuimage.CommandFlags{Builder: "bb"},
		ArchiveFormat: "cpio",
		OutputFile:    defaultFile(env),
	}
	tf := &mkuimage.TemplateFlags{}

	makeCmd := cli.Command{
		Name:  "make",
		Short: "create uimage from specified flags",
		Run: func(args []string) {
			// Set defaults.
			m := []uimage.Modifier{
				uimage.WithReplaceEnv(env),
				uimage.WithBaseArchive(uimage.DefaultRamfs()),
				uimage.WithCPIOOutput(defaultFile(env)),
			}
			if err := mkuimage.CreateUimage(l, m, tf, f, args); err != nil {
				l.Errorf("mkuimage error: %v", err)
				os.Exit(1)
			}
		},
	}
	l.RegisterVerboseFlag(makeCmd.Flags(), "v", slog.LevelDebug)
	f.RegisterFlags(makeCmd.Flags())
	tf.RegisterFlags(makeCmd.Flags())

	listconfigsCmd := cli.Command{
		Name:  "listconfigs",
		Short: "list template configs",
		Run: func(args []string) {
			tpl, err := tf.Get()
			if err != nil {
				l.Errorf("Failed to get template: %w", err)
				os.Exit(1)
			}
			configs := maps.Keys(tpl.Configs)
			slices.Sort(configs)
			for _, name := range configs {
				fmt.Println(name)
			}
		},
	}
	l.RegisterVerboseFlag(listconfigsCmd.Flags(), "v", slog.LevelDebug)
	tf.RegisterFlags(listconfigsCmd.Flags())

	listCmd := cli.Command{
		Name:  "list",
		Short: "list commands from template (no args: lists all cmds in template)",
		Run: func(args []string) {
			tpl, err := tf.Get()
			if err != nil {
				l.Errorf("Failed to get template: %w", err)
				os.Exit(1)
			}
			var cmds []string
			if tf.Config == "" && len(args) == 0 {
				for _, conf := range tpl.Configs {
					for _, c := range conf.Commands {
						cmds = append(cmds, tpl.CommandsFor(c.Commands...)...)
					}
				}
				for _, c := range tpl.Commands {
					cmds = append(cmds, c...)
				}
			}
			if tf.Config != "" {
				if _, ok := tpl.Configs[tf.Config]; !ok {
					l.Errorf("Config %s not found", tf.Config)
					os.Exit(1)
				}
				for _, c := range tpl.Configs[tf.Config].Commands {
					cmds = append(cmds, tpl.CommandsFor(c.Commands...)...)
				}
			}
			cmds = append(cmds, tpl.CommandsFor(args...)...)

			lookupEnv := findpkg.DefaultEnv()
			paths, err := findpkg.ResolveGlobs(l.AtLevel(slog.LevelInfo), env, lookupEnv, cmds)
			if err != nil {
				l.Errorf("Failed to resolve commands: %v", err)
				os.Exit(1)
			}
			uniquePaths := map[string]struct{}{}
			for _, p := range paths {
				uniquePaths[p] = struct{}{}
			}
			ps := maps.Keys(uniquePaths)
			slices.Sort(ps)
			for _, p := range ps {
				fmt.Println(p)
			}
		},
	}
	l.RegisterVerboseFlag(listCmd.Flags(), "v", slog.LevelDebug)
	tf.RegisterFlags(listCmd.Flags())

	app := cli.App{makeCmd, listconfigsCmd, listCmd}
	app.Run(os.Args)
}

func defaultFile(env *golang.Environ) string {
	if len(env.GOOS) == 0 || len(env.GOARCH) == 0 {
		return "/tmp/initramfs.cpio"
	}
	return fmt.Sprintf("/tmp/initramfs.%s_%s.cpio", env.GOOS, env.GOARCH)
}
