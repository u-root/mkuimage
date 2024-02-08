// Copyright 2015-2017 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package initramfs

import (
	"fmt"
	"io"
	"os"

	"github.com/u-root/gobusybox/src/pkg/golang"
	"github.com/u-root/mkuimage/cpio"
)

// CPIOArchiver is an implementation of Archiver for the cpio format.
type CPIOArchiver struct {
	cpio.RecordFormat
}

// CreateDefault chooses /tmp/initramfs.cpio or /tmp/initramfs.GOOS_GOARCH.cpio
// if available.
func (ca CPIOArchiver) CreateDefault(env *golang.Environ) (string, error) {
	if len(env.GOOS) == 0 || len(env.GOARCH) == 0 {
		return "/tmp/initramfs.cpio", nil
	}
	return fmt.Sprintf("/tmp/initramfs.%s_%s.cpio", env.GOOS, env.GOARCH), nil
}

// OpenWriter opens `path` as the correct file type and returns an
// Writer pointing to `path`.
func (ca CPIOArchiver) OpenWriter(path string) (Writer, error) {
	if len(path) == 0 {
		return nil, fmt.Errorf("path is required")
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, err
	}
	return osWriter{ca.RecordFormat.Writer(f), f}, nil
}

// osWriter implements Writer.
type osWriter struct {
	cpio.RecordWriter

	f *os.File
}

// Finish implements Writer.Finish.
func (o osWriter) Finish() error {
	err := cpio.WriteTrailer(o)
	o.f.Close()
	return err
}

// Reader implements Archiver.Reader.
func (ca CPIOArchiver) Reader(r io.ReaderAt) Reader {
	return ca.RecordFormat.Reader(r)
}
