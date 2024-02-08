// Copyright 2018 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package initramfs

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/u-root/gobusybox/src/pkg/golang"
	"github.com/u-root/mkuimage/cpio"
)

// DirArchiver implements Archiver for a directory.
type DirArchiver struct{}

// Reader implements Archiver.Reader.
//
// Currently unsupported for directories.
func (da DirArchiver) Reader(io.ReaderAt) Reader {
	return nil
}

// CreateDefault creates a tmpdir using os.MkdirTemp prefixed with mkuimage- or
// mkuimage-GOOS-GOARCH- if available.
func (da DirArchiver) CreateDefault(env *golang.Environ) (string, error) {
	name := "mkuimage-"
	if len(env.GOOS) != 0 && len(env.GOARCH) != 0 {
		name = fmt.Sprintf("mkuimage-%s-%s-", env.GOOS, env.GOARCH)
	}
	return os.MkdirTemp("", name)
}

// OpenWriter implements Archiver.OpenWriter.
func (da DirArchiver) OpenWriter(path string) (Writer, error) {
	if len(path) == 0 {
		return nil, fmt.Errorf("path is required")
	}
	if err := os.MkdirAll(path, 0o755); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, err
	}
	return dirWriter{path}, nil
}

// dirWriter implements Writer.
type dirWriter struct {
	dir string
}

// WriteRecord implements Writer.WriteRecord.
func (dw dirWriter) WriteRecord(r cpio.Record) error {
	return cpio.CreateFileInRoot(r, dw.dir, false)
}

// Finish implements Writer.Finish.
func (dw dirWriter) Finish() error {
	return nil
}
