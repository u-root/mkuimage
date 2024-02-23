// Copyright 2024 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mkuimage

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/u-root/mkuimage/uimage"
	"github.com/u-root/mkuimage/uimage/builder"
	"github.com/u-root/uio/llog"
)

var recommendedVersions = []string{
	"go1.20",
	"go1.21",
	"go1.22",
}

func isRecommendedVersion(v string) bool {
	for _, r := range recommendedVersions {
		if strings.HasPrefix(v, r) {
			return true
		}
	}
	return false
}

// CreateUimage creates a uimage from the given base modifiers and flags.
func CreateUimage(l *llog.Logger, base []uimage.Modifier, f *Flags, args []string) error {
	keepTempDir := f.KeepTempDir
	if f.TempDir == "" {
		var err error
		f.TempDir, err = os.MkdirTemp("", "u-root")
		if err != nil {
			return err
		}
		defer func() {
			if keepTempDir {
				l.Infof("Keeping temp dir %s", f.TempDir)
			} else {
				os.RemoveAll(f.TempDir)
			}
		}()
	} else if _, err := os.Stat(f.TempDir); os.IsNotExist(err) {
		if err := os.MkdirAll(f.TempDir, 0o755); err != nil {
			return fmt.Errorf("temporary directory %q did not exist; tried to mkdir but failed: %v", f.TempDir, err)
		}
	}

	// Set defaults.
	more, err := f.Modifiers(args...)
	if err != nil {
		return err
	}

	opts, err := uimage.OptionsFor(append(base, more...)...)
	if err != nil {
		return err
	}

	env := opts.Env

	l.Infof("Build environment: %s", env)
	if env.GOOS != "linux" {
		l.Warnf("GOOS is not linux. Did you mean to set GOOS=linux?")
	}
	v, err := env.Version()
	if err != nil {
		l.Infof("Could not get environment's Go version, using runtime's version: %v", err)
		v = runtime.Version()
	}
	if !isRecommendedVersion(v) {
		l.Warnf(`You are not using one of the recommended Go versions (have = %s, recommended = %v).
			Some packages may not compile.
			Go to https://golang.org/doc/install to find out how to install a newer version of Go,
			or use https://godoc.org/golang.org/dl/%s to install an additional version of Go.`,
			v, recommendedVersions, recommendedVersions[0])
	}

	err = opts.Create(l)
	if errors.Is(err, builder.ErrBusyboxFailed) {
		l.Errorf("Preserving temp dir due to busybox build error")
		keepTempDir = true
	}
	return err
}
