// Copyright 2024 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mkuimage

import (
	"errors"
	"flag"
	"os"
	"testing"
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
