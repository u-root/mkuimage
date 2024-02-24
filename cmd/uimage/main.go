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

	"github.com/u-root/gobusybox/src/pkg/golang"
	"github.com/u-root/mkuimage/uimage"
	"github.com/u-root/mkuimage/uimage/mkuimage"
	"github.com/u-root/uio/cli"
	"github.com/u-root/uio/llog"
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

	app := cli.App{makeCmd}
	app.Run(os.Args)
}

func defaultFile(env *golang.Environ) string {
	if len(env.GOOS) == 0 || len(env.GOARCH) == 0 {
		return "/tmp/initramfs.cpio"
	}
	return fmt.Sprintf("/tmp/initramfs.%s_%s.cpio", env.GOOS, env.GOARCH)
}
