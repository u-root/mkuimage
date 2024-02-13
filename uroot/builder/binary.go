// Copyright 2015-2017 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package builder

import (
	"fmt"
	"path/filepath"
	"sync"

	"github.com/u-root/gobusybox/src/pkg/golang"
	"github.com/u-root/mkuimage/uroot/initramfs"
	"github.com/u-root/uio/llog"
	"golang.org/x/tools/go/packages"
)

func dirFor(env *golang.Environ, pkg string) (string, error) {
	pkgs, err := env.Lookup(packages.NeedName|packages.NeedFiles, "", pkg)
	if err != nil {
		return "", fmt.Errorf("failed to look up package %q: %v", pkg, err)
	}

	// One directory = one package in standard Go, so
	// finding the first file's parent directory should
	// find us the package directory.
	var dir string
	for _, p := range pkgs {
		if len(p.GoFiles) > 0 {
			dir = filepath.Dir(p.GoFiles[0])
		}
	}
	if dir == "" {
		return "", fmt.Errorf("%w for %q", ErrNoGoFiles, pkg)
	}
	return dir, nil
}

// BinaryBuilder builds each Go command as a separate binary.
//
// BinaryBuilder is an implementation of Builder.
type BinaryBuilder struct{}

// DefaultBinaryDir implements Builder.DefaultBinaryDir.
//
// "bin" is the default initramfs binary directory for these binaries.
func (BinaryBuilder) DefaultBinaryDir() string {
	return "bin"
}

// Build implements Builder.Build.
func (b BinaryBuilder) Build(l *llog.Logger, af *initramfs.Files, opts Opts) error {
	if opts.Env == nil {
		return ErrEnvMissing
	}
	if opts.TempDir == "" {
		return ErrTempDirMissing
	}
	binaryDir := opts.BinaryDir
	if binaryDir == "" {
		binaryDir = b.DefaultBinaryDir()
	}

	result := make(chan error, len(opts.Packages))

	var wg sync.WaitGroup
	for _, pkg := range opts.Packages {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			dir, err := dirFor(opts.Env, p)
			if err != nil {
				result <- err
				return
			}
			result <- opts.Env.BuildDir(
				dir,
				filepath.Join(opts.TempDir, binaryDir, filepath.Base(p)),
				opts.BuildOpts)
		}(pkg)
	}

	wg.Wait()
	close(result)

	for err := range result {
		if err != nil {
			return err
		}
	}

	// Add bin directory to archive.
	return af.AddFile(opts.TempDir, "")
}
