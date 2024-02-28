// Copyright 2024 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// gentpldeps generates a command dependency Go file.
//
// Usage:
//   - go get $module...
//   - create .mkuimage.yaml file with commands
//   - gentpldeps -t <tag> -o deps.go -p <pkg>
package main

import (
	"bytes"
	"flag"
	"log"
	"os"
	"sort"
	"text/template"

	"github.com/u-root/gobusybox/src/pkg/bb/findpkg"
	"github.com/u-root/gobusybox/src/pkg/golang"
	"github.com/u-root/mkuimage/uimage/mkuimage"
	"golang.org/x/exp/maps"
)

var (
	pkg = flag.String("p", "", "Package")
	o   = flag.String("o", "", "Output file name")
	tag = flag.String("t", "", "Go build tag for the file")
)

func main() {
	tf := &mkuimage.TemplateFlags{}
	tf.RegisterFlags(flag.CommandLine)
	flag.Parse()

	if *pkg == "" {
		log.Fatal("Must specify package name")
	}
	if *tag == "" {
		log.Fatal("Must specify Go build tag")
	}
	if *o == "" {
		log.Fatal("Must specify output file name")
	}

	tpls, err := tf.Get()
	if err != nil {
		log.Fatal(err)
	}
	if tpls == nil {
		log.Fatalf("No template found")
	}

	var cmds []string
	for _, c := range tpls.Commands {
		cmds = append(cmds, c...)
	}
	for _, conf := range tpls.Configs {
		for _, c := range conf.Commands {
			cmds = append(cmds, tpls.CommandsFor(c.Commands...)...)
		}
	}
	paths, err := findpkg.ResolveGlobs(nil, golang.Default(), findpkg.DefaultEnv(), cmds)
	if err != nil {
		log.Fatal(err)
	}
	dedup := map[string]struct{}{}
	for _, p := range paths {
		dedup[p] = struct{}{}
	}
	c := maps.Keys(dedup)
	sort.Strings(c)

	tpl := `//go:build {{.Tag}}

package {{.Package}}

import ({{range .Imports}}
	_ "{{.}}"{{end}}
)
`

	vars := struct {
		Tag     string
		Package string
		Imports []string
	}{
		Tag:     *tag,
		Package: *pkg,
		Imports: c,
	}
	t := template.Must(template.New("tpl").Parse(tpl))
	var b bytes.Buffer
	if err := t.Execute(&b, vars); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*o, b.Bytes(), 0o644); err != nil {
		log.Fatal(err)
	}
}
