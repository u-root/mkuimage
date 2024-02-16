// Copyright 2018 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package initramfs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/u-root/mkuimage/cpio"
	"github.com/u-root/uio/uio"
)

func archive(tb testing.TB, rs ...cpio.Record) *cpio.Archive {
	tb.Helper()
	a, err := cpio.ArchiveFromRecords(rs)
	if err != nil {
		tb.Fatal(err)
	}
	return a
}

func TestFilesAddFileNoFollow(t *testing.T) {
	regularFile, err := os.CreateTemp("", "archive-files-add-file")
	if err != nil {
		t.Error(err)
	}
	defer os.RemoveAll(regularFile.Name())

	dir := t.TempDir()
	dir2 := t.TempDir()

	//nolint:errcheck
	{
		os.Create(filepath.Join(dir, "foo2"))
		os.Symlink(filepath.Join(dir, "foo2"), filepath.Join(dir2, "foo3"))
	}

	for i, tt := range []struct {
		name   string
		af     *Files
		src    string
		dest   string
		result *Files
		err    error
	}{
		{
			name: "just add a file",
			af:   NewFiles(),

			src:  regularFile.Name(),
			dest: "bar/foo",

			result: &Files{
				Files: map[string]string{
					"bar/foo": regularFile.Name(),
				},
				Records: map[string]cpio.Record{},
			},
		},
		{
			name: "add symlinked file, NOT following",
			af:   NewFiles(),
			src:  filepath.Join(dir2, "foo3"),
			dest: "bar/foo",
			result: &Files{
				Files: map[string]string{
					"bar/foo": filepath.Join(dir2, "foo3"),
				},
				Records: map[string]cpio.Record{},
			},
		},
	} {
		t.Run(fmt.Sprintf("Test %02d: %s", i, tt.name), func(t *testing.T) {
			err := tt.af.AddFileNoFollow(tt.src, tt.dest)
			if !errors.Is(err, tt.err) {
				t.Errorf("AddFileNoFollow = %v, want %v", err, tt.err)
			}

			if tt.result != nil && !reflect.DeepEqual(tt.af, tt.result) {
				t.Errorf("got %v, want %v", tt.af, tt.result)
			}
		})
	}
}

func TestFilesAddFile(t *testing.T) {
	regularFile, err := os.CreateTemp("", "archive-files-add-file")
	if err != nil {
		t.Error(err)
	}
	defer os.RemoveAll(regularFile.Name())

	dir := t.TempDir()
	dir2 := t.TempDir()
	dir3 := t.TempDir()

	symlinkToDir3 := filepath.Join(dir3, "fooSymDir/")
	fooDir := filepath.Join(dir3, "fooDir")
	_ = os.WriteFile(filepath.Join(dir, "foo"), nil, 0o777)
	_ = os.WriteFile(filepath.Join(dir, "foo2"), nil, 0o777)
	_ = os.Symlink(filepath.Join(dir, "foo2"), filepath.Join(dir2, "foo3"))

	_ = os.Mkdir(fooDir, os.ModePerm)
	_ = os.Symlink(fooDir, symlinkToDir3)
	_ = os.WriteFile(filepath.Join(fooDir, "foo"), nil, 0o777)
	_ = os.WriteFile(filepath.Join(fooDir, "bar"), nil, 0o777)

	for i, tt := range []struct {
		name   string
		af     *Files
		src    string
		dest   string
		result *Files
		err    error
	}{
		{
			name: "just add a file",
			af:   NewFiles(),

			src:  regularFile.Name(),
			dest: "bar/foo",

			result: &Files{
				Files: map[string]string{
					"bar/foo": regularFile.Name(),
				},
				Records: map[string]cpio.Record{},
			},
		},
		{
			name: "add symlinked file, following",
			af:   NewFiles(),
			src:  filepath.Join(dir2, "foo3"),
			dest: "bar/foo",
			result: &Files{
				Files: map[string]string{
					"bar/foo": filepath.Join(dir, "foo2"),
				},
				Records: map[string]cpio.Record{},
			},
		},
		{
			name: "add symlinked directory, following",
			af:   NewFiles(),
			src:  symlinkToDir3,
			dest: "foo/",
			result: &Files{
				Files: map[string]string{
					"foo":     fooDir,
					"foo/foo": filepath.Join(fooDir, "foo"),
					"foo/bar": filepath.Join(fooDir, "bar"),
				},
				Records: map[string]cpio.Record{},
			},
		},
		{
			name: "add file that exists in Files",
			af: &Files{
				Files: map[string]string{
					"bar/foo": "/some/other/place",
				},
			},
			src:  regularFile.Name(),
			dest: "bar/foo",
			result: &Files{
				Files: map[string]string{
					"bar/foo": "/some/other/place",
				},
			},
			err: os.ErrExist,
		},
		{
			name: "add a file that exists in Records",
			af: &Files{
				Records: map[string]cpio.Record{
					"bar/foo": cpio.Symlink("bar/foo", "/some/other/place"),
				},
			},
			src:  regularFile.Name(),
			dest: "bar/foo",
			result: &Files{
				Records: map[string]cpio.Record{
					"bar/foo": cpio.Symlink("bar/foo", "/some/other/place"),
				},
			},
			err: os.ErrExist,
		},
		{
			name: "add a file that already exists in Files, but is the same one",
			af: &Files{
				Files: map[string]string{
					"bar/foo": regularFile.Name(),
				},
			},
			src:  regularFile.Name(),
			dest: "bar/foo",
			result: &Files{
				Files: map[string]string{
					"bar/foo": regularFile.Name(),
				},
			},
		},
		{
			name: "absolute destination paths are made relative",
			af: &Files{
				Files: map[string]string{},
			},
			src:  dir,
			dest: "/bar/foo",
			result: &Files{
				Files: map[string]string{
					"bar/foo":      dir,
					"bar/foo/foo":  filepath.Join(dir, "foo"),
					"bar/foo/foo2": filepath.Join(dir, "foo2"),
				},
			},
		},
		{
			name: "add a directory",
			af: &Files{
				Files: map[string]string{},
			},
			src:  dir,
			dest: "bar/foo",
			result: &Files{
				Files: map[string]string{
					"bar/foo":      dir,
					"bar/foo/foo":  filepath.Join(dir, "foo"),
					"bar/foo/foo2": filepath.Join(dir, "foo2"),
				},
			},
		},
		{
			name: "add a different directory to the same destination, no overlapping children",
			af: &Files{
				Files: map[string]string{
					"bar/foo":     "/some/place/real",
					"bar/foo/zed": "/some/place/real/zed",
				},
			},
			src:  dir,
			dest: "bar/foo",
			result: &Files{
				Files: map[string]string{
					"bar/foo":      dir,
					"bar/foo/foo":  filepath.Join(dir, "foo"),
					"bar/foo/foo2": filepath.Join(dir, "foo2"),
					"bar/foo/zed":  "/some/place/real/zed",
				},
			},
		},
		{
			name: "add a different directory to the same destination, overlapping children",
			af: &Files{
				Files: map[string]string{
					"bar/foo":      "/some/place/real",
					"bar/foo/foo2": "/some/place/real/zed",
				},
			},
			src:  dir,
			dest: "bar/foo",
			err:  os.ErrExist,
		},
	} {
		t.Run(fmt.Sprintf("Test %02d: %s", i, tt.name), func(t *testing.T) {
			err := tt.af.AddFile(tt.src, tt.dest)
			if !errors.Is(err, tt.err) {
				t.Errorf("AddFile = %v, want %v", err, tt.err)
			}

			if tt.result != nil && !reflect.DeepEqual(tt.af, tt.result) {
				t.Errorf("got %v, want %v", tt.af, tt.result)
			}
		})
	}
}

func TestFilesAddRecord(t *testing.T) {
	for i, tt := range []struct {
		af     *Files
		record cpio.Record

		result *Files
		err    error
	}{
		{
			af:     NewFiles(),
			record: cpio.Symlink("bar/foo", ""),
			result: &Files{
				Files: map[string]string{},
				Records: map[string]cpio.Record{
					"bar/foo": cpio.Symlink("bar/foo", ""),
				},
			},
		},
		{
			af: &Files{
				Files: map[string]string{
					"bar/foo": "/some/other/place",
				},
			},
			record: cpio.Symlink("bar/foo", ""),
			result: &Files{
				Files: map[string]string{
					"bar/foo": "/some/other/place",
				},
			},
			err: os.ErrExist,
		},
		{
			af: &Files{
				Records: map[string]cpio.Record{
					"bar/foo": cpio.Symlink("bar/foo", "/some/other/place"),
				},
			},
			record: cpio.Symlink("bar/foo", ""),
			result: &Files{
				Records: map[string]cpio.Record{
					"bar/foo": cpio.Symlink("bar/foo", "/some/other/place"),
				},
			},
			err: os.ErrExist,
		},
		{
			af: &Files{
				Records: map[string]cpio.Record{
					"bar/foo": cpio.Symlink("bar/foo", "/some/other/place"),
				},
			},
			record: cpio.Symlink("bar/foo", "/some/other/place"),
			result: &Files{
				Records: map[string]cpio.Record{
					"bar/foo": cpio.Symlink("bar/foo", "/some/other/place"),
				},
			},
		},
		{
			record: cpio.Symlink("/bar/foo", ""),
			err:    errAbsoluteName,
		},
	} {
		t.Run(fmt.Sprintf("Test %02d", i), func(t *testing.T) {
			err := tt.af.AddRecord(tt.record)
			if !errors.Is(err, tt.err) {
				t.Errorf("AddRecord = %v, want %v", err, tt.err)
			}

			if !reflect.DeepEqual(tt.af, tt.result) {
				t.Errorf("got %v, want %v", tt.af, tt.result)
			}
		})
	}
}

func TestFilesfillInParent(t *testing.T) {
	for i, tt := range []struct {
		af     *Files
		result *Files
	}{
		{
			af: &Files{
				Records: map[string]cpio.Record{
					"foo/bar": cpio.Directory("foo/bar", 0o777),
				},
			},
			result: &Files{
				Records: map[string]cpio.Record{
					"foo/bar": cpio.Directory("foo/bar", 0o777),
					"foo":     cpio.Directory("foo", 0o755),
				},
			},
		},
		{
			af: &Files{
				Files: map[string]string{
					"baz/baz/baz": "/somewhere",
				},
				Records: map[string]cpio.Record{
					"foo/bar": cpio.Directory("foo/bar", 0o777),
				},
			},
			result: &Files{
				Files: map[string]string{
					"baz/baz/baz": "/somewhere",
				},
				Records: map[string]cpio.Record{
					"foo/bar": cpio.Directory("foo/bar", 0o777),
					"foo":     cpio.Directory("foo", 0o755),
					"baz":     cpio.Directory("baz", 0o755),
					"baz/baz": cpio.Directory("baz/baz", 0o755),
				},
			},
		},
		{
			af:     &Files{},
			result: &Files{},
		},
	} {
		t.Run(fmt.Sprintf("Test %02d", i), func(t *testing.T) {
			tt.af.fillInParents()
			if !reflect.DeepEqual(tt.af, tt.result) {
				t.Errorf("got %v, want %v", tt.af, tt.result)
			}
		})
	}
}

type Records map[string]cpio.Record

func recordsEqual(r1, r2 Records, recordEqual func(cpio.Record, cpio.Record) bool) bool {
	for name, s1 := range r1 {
		s2, ok := r2[name]
		if !ok {
			return false
		}
		if !recordEqual(s1, s2) {
			return false
		}
	}
	for name := range r2 {
		if _, ok := r1[name]; !ok {
			return false
		}
	}
	return true
}

func sameNameModeContent(r1 cpio.Record, r2 cpio.Record) bool {
	if r1.Name != r2.Name || r1.Mode != r2.Mode {
		return false
	}
	return uio.ReaderAtEqual(r1.ReaderAt, r2.ReaderAt)
}

func TestOptsWrite(t *testing.T) {
	for i, tt := range []struct {
		desc   string
		opts   *Opts
		output *cpio.Archive
		want   Records
		err    error
	}{
		{
			desc: "no conflicts, just records",
			opts: &Opts{
				Files: &Files{
					Records: map[string]cpio.Record{
						"foo": cpio.Symlink("foo", "elsewhere"),
					},
				},
				BaseArchive: &Archive{Archive: archive(t,
					cpio.Directory("etc", 0o777),
					cpio.Directory("etc/nginx", 0o777),
				)},
			},
			output: &cpio.Archive{
				Files: make(map[string]cpio.Record),
			},
			want: Records{
				"foo":        cpio.Symlink("foo", "elsewhere"),
				"etc":        cpio.Directory("etc", 0o777),
				"etc/nginx":  cpio.Directory("etc/nginx", 0o777),
				cpio.Trailer: cpio.TrailerRecord,
			},
		},
		{
			desc: "default already exists",
			opts: &Opts{
				Files: &Files{
					Records: map[string]cpio.Record{
						"etc": cpio.Symlink("etc", "whatever"),
					},
				},
				BaseArchive: &Archive{Archive: archive(t,
					cpio.Directory("etc", 0o777),
				)},
			},
			output: &cpio.Archive{
				Files: make(map[string]cpio.Record),
			},
			want: Records{
				"etc":        cpio.Symlink("etc", "whatever"),
				cpio.Trailer: cpio.TrailerRecord,
			},
		},
		{
			desc: "no conflicts, missing parent automatically created",
			opts: &Opts{
				Files: &Files{
					Records: map[string]cpio.Record{
						"foo/bar/baz": cpio.Symlink("foo/bar/baz", "elsewhere"),
					},
				},
				BaseArchive: nil,
			},
			output: &cpio.Archive{
				Files: make(map[string]cpio.Record),
			},
			want: Records{
				"foo":         cpio.Directory("foo", 0o755),
				"foo/bar":     cpio.Directory("foo/bar", 0o755),
				"foo/bar/baz": cpio.Symlink("foo/bar/baz", "elsewhere"),
				cpio.Trailer:  cpio.TrailerRecord,
			},
		},
		{
			desc: "parent only automatically created if not already exists",
			opts: &Opts{
				Files: &Files{
					Records: map[string]cpio.Record{
						"foo/bar":     cpio.Directory("foo/bar", 0o444),
						"foo/bar/baz": cpio.Symlink("foo/bar/baz", "elsewhere"),
					},
				},
				BaseArchive: nil,
			},
			output: &cpio.Archive{
				Files: make(map[string]cpio.Record),
			},
			want: Records{
				"foo":         cpio.Directory("foo", 0o755),
				"foo/bar":     cpio.Directory("foo/bar", 0o444),
				"foo/bar/baz": cpio.Symlink("foo/bar/baz", "elsewhere"),
				cpio.Trailer:  cpio.TrailerRecord,
			},
		},
		{
			desc: "base archive",
			opts: &Opts{
				Files: &Files{
					Records: map[string]cpio.Record{
						"foo/bar": cpio.Symlink("foo/bar", "elsewhere"),
						"exists":  cpio.Directory("exists", 0o777),
					},
				},
				BaseArchive: &Archive{Archive: archive(t,
					cpio.Directory("etc", 0o755),
					cpio.Directory("foo", 0o444),
					cpio.Directory("exists", 0),
				)},
			},
			output: &cpio.Archive{
				Files: make(map[string]cpio.Record),
			},
			want: Records{
				"etc":        cpio.Directory("etc", 0o755),
				"exists":     cpio.Directory("exists", 0o777),
				"foo":        cpio.Directory("foo", 0o444),
				"foo/bar":    cpio.Symlink("foo/bar", "elsewhere"),
				cpio.Trailer: cpio.TrailerRecord,
			},
		},
		{
			desc: "base archive with init, no user init",
			opts: &Opts{
				Files: &Files{
					Records: map[string]cpio.Record{},
				},
				BaseArchive: &Archive{Archive: archive(t,
					cpio.StaticFile("init", "boo", 0o555),
				)},
			},
			output: &cpio.Archive{
				Files: make(map[string]cpio.Record),
			},
			want: Records{
				"init":       cpio.StaticFile("init", "boo", 0o555),
				cpio.Trailer: cpio.TrailerRecord,
			},
		},
		{
			desc: "base archive with init and user init",
			opts: &Opts{
				Files: &Files{
					Records: map[string]cpio.Record{
						"init": cpio.StaticFile("init", "bar", 0o444),
					},
				},
				BaseArchive: &Archive{Archive: archive(t,
					cpio.StaticFile("init", "boo", 0o555),
				)},
			},
			output: &cpio.Archive{
				Files: make(map[string]cpio.Record),
			},
			want: Records{
				"init":       cpio.StaticFile("init", "bar", 0o444),
				"inito":      cpio.StaticFile("inito", "boo", 0o555),
				cpio.Trailer: cpio.TrailerRecord,
			},
		},
		{
			desc: "base archive with init, use existing init",
			opts: &Opts{
				Files: &Files{
					Records: map[string]cpio.Record{},
				},
				BaseArchive: &Archive{Archive: archive(t,
					cpio.StaticFile("init", "boo", 0o555),
				)},
				UseExistingInit: true,
			},
			output: &cpio.Archive{
				Files: make(map[string]cpio.Record),
			},
			want: Records{
				"init":       cpio.StaticFile("init", "boo", 0o555),
				cpio.Trailer: cpio.TrailerRecord,
			},
		},
		{
			desc: "base archive with init and user init, use existing init",
			opts: &Opts{
				Files: &Files{
					Records: map[string]cpio.Record{
						"init": cpio.StaticFile("init", "huh", 0o111),
					},
				},
				BaseArchive: &Archive{Archive: archive(t,
					cpio.StaticFile("init", "boo", 0o555),
				)},
				UseExistingInit: true,
			},
			output: &cpio.Archive{
				Files: make(map[string]cpio.Record),
			},
			want: Records{
				"init":       cpio.StaticFile("init", "boo", 0o555),
				"inito":      cpio.StaticFile("inito", "huh", 0o111),
				cpio.Trailer: cpio.TrailerRecord,
			},
		},
		{
			desc: "base archive with init and extra records conflict",
			opts: &Opts{
				Files: &Files{
					Records: map[string]cpio.Record{
						"init": cpio.StaticFile("init", "huh", 0o111),
					},
				},
				BaseArchive: &Archive{Archive: archive(t,
					cpio.StaticFile("init", "boo", 0o555),
				)},
				Records: []cpio.Record{
					cpio.StaticFile("init", "huh", 0o111),
				},
				UseExistingInit: true,
			},
			errs: []error{os.ErrExist},
			output: &cpio.Archive{
				Files: make(map[string]cpio.Record),
			},
		},
		{
			desc: "extra records",
			opts: &Opts{
				Files: &Files{
					Records: map[string]cpio.Record{
						"init": cpio.StaticFile("init", "huh", 0o111),
					},
				},
				Records: []cpio.Record{
					cpio.StaticFile("etc/foo", "huh", 0o111),
				},
			},
			output: &cpio.Archive{
				Files: make(map[string]cpio.Record),
			},
			want: Records{
				"init":       cpio.StaticFile("init", "boo", 0o555),
				"etc/foo":    cpio.StaticFile("etc/foo", "huh", 0o111),
				cpio.Trailer: cpio.TrailerRecord,
			},
		},
	} {
		t.Run(fmt.Sprintf("Test %02d (%s)", i, tt.desc), func(t *testing.T) {
			tt.opts.OutputFile = &Archive{tt.output}

			if err := Write(tt.opts); !errors.Is(err, tt.err) {
				t.Errorf("Write = %v, want %v", err, tt.err)
			}

			if !recordsEqual(tt.output.Files, tt.want, sameNameModeContent) {
				t.Errorf("Write() = %v, want %v", tt.output.Files, tt.want)
			}
		})
	}
}
