# uimage

[![GoDoc](https://pkg.go.dev/badge/github.com/u-root/mkuimage)](https://pkg.go.dev/github.com/u-root/mkuimage)
[![codecov](https://codecov.io/gh/u-root/mkuimage/graph/badge.svg?token=5Z9B3OyVYi)](https://codecov.io/gh/u-root/mkuimage)
[![Slack](https://slack.osfw.dev/badge.svg)](https://slack.osfw.dev)
[![License](https://img.shields.io/badge/License-BSD%203--Clause-blue.svg)](https://github.com/u-root/mkuimage/blob/main/LICENSE)

uimage builds initramfs images composed of arbitrary Go commands and files.

uimage optimizes for space by utilizing
[gobusybox](https://github.com/u-root/gobusybox) to compile many arbitrary Go
commands into one binary.

uimage can be easily used with [u-root](https://github.com/u-root/u-root),
which contains many Go coreutils-like commands as well as bootloaders. However,
uimage supports compilation of any Go command, and the use of u-root is not
required.

## Getting Started

Make sure your Go version is >= 1.21. If your Go version is lower,

```shell
$ go install golang.org/dl/go1.21.5@latest
$ go1.21.5 download
$ go1.21.5 version
# Now use go1.21.5 in place of go
```

Download and install mkuimage either via git:

```shell
git clone https://github.com/u-root/mkuimage
cd mkuimage/cmd/mkuimage
go install
```

Or install directly with go:

```shell
go install github.com/u-root/mkuimage/cmd/mkuimage@latest
```

> [!NOTE]
> The `mkuimage` command will end up in `$GOPATH/bin/mkuimage`, so you may
> need to add `$GOPATH/bin` to your `$PATH`.

## Examples

Here are some examples of using the `mkuimage` command to build an initramfs.

```shell
git clone https://github.com/u-root/u-root
```

Build gobusybox binaries of these two commands and add to initramfs:

```shell
$ cd ./u-root
$ mkuimage ./cmds/core/{init,gosh}

$ cpio -ivt < /tmp/initramfs.linux_amd64.cpio
...
-rwxr-x---   0 root     root      4542464 Jan  1  1970 bbin/bb
lrwxrwxrwx   0 root     root            2 Jan  1  1970 bbin/gosh -> bb
lrwxrwxrwx   0 root     root            2 Jan  1  1970 bbin/init -> bb
...
```

Add symlinks for shell and init:

```shell
$ mkuimage -initcmd=init -defaultsh=gosh ./cmds/core/{init,gosh}

$ cpio -ivt < /tmp/initramfs.linux_amd64.cpio
...
lrwxrwxrwx   0 root     root           12 Jan  1  1970 bin/defaultsh -> ../bbin/gosh
lrwxrwxrwx   0 root     root           12 Jan  1  1970 bin/sh -> ../bbin/gosh
...
lrwxrwxrwx   0 root     root            9 Jan  1  1970 init -> bbin/init
...
```

### Builds with globs and exclusion

Build everything from core without ls and losetup:

```shell
$ mkuimage ./cmds/core/* -./cmds/core/{ls,losetup}
```

### Multi-module workspace builds

> [!IMPORTANT]
>
> `mkuimage` works when `go build` and `go list` work as well.

There are 2 ways to build multi-module command images using standard Go tooling.

```shell
$ mkdir workspace
$ cd workspace
$ git clone https://github.com/u-root/u-root
$ git clone https://github.com/u-root/cpu

$ go work init ./u-root
$ go work use ./cpu

$ mkuimage ./u-root/cmds/core/{init,gosh} ./cpu/cmds/cpud

$ cpio -ivt < /tmp/initramfs.linux_amd64.cpio
...
-rwxr-x---   0 root     root      6365184 Jan  1  1970 bbin/bb
lrwxrwxrwx   0 root     root            2 Jan  1  1970 bbin/cpud -> bb
lrwxrwxrwx   0 root     root            2 Jan  1  1970 bbin/gosh -> bb
lrwxrwxrwx   0 root     root            2 Jan  1  1970 bbin/init -> bb
...

# Works for offline vendored builds as well.
$ go work vendor # Go 1.22 feature.

$ mkuimage ./u-root/cmds/core/{init,gosh} ./cpu/cmds/cpud
```

`GBB_PATH` is a place that mkuimage will look for commands. Each colon-separated
`GBB_PATH` element is concatenated with patterns from the command-line and
checked for existence. For example:

```shell
GBB_PATH=$(pwd)/u-root:$(pwd)/cpu mkuimage \
    cmds/core/{init,gosh} \
    cmds/cpud

# Matches
#   ./u-root/cmds/core/{init,gosh}
#   ./cpu/cmds/cpud
```

To ease usability, the `goanywhere` tool can create one Go workspaces the fly.
This works **only with local file system paths**:

```shell
$ go install github.com/u-root/gobusybox/src/cmd/goanywhere@latest

$ goanywhere ./u-root/cmds/core/{init,gosh} ./cpu/cmds/cpud -- go build -o $(pwd)
$ goanywhere ./u-root/cmds/core/{init,gosh} ./cpu/cmds/cpud -- mkuimage
```

`goanywhere` creates a workspace in a temporary directory with the given
modules, and then execs `u-root` in the workspace passing along the command
names.

`goanywhere` supports `GBB_PATH`, exclusions, globs, and curly brace expansions
as well.

> [!CAUTION]
>
> While workspaces are good for local compilation, they are not meant to be
> checked in to version control systems. See below for the recommended way.

### Multi-module go.mod builds

You may also create a go.mod with the commands you intend to compile.

To depend on commands outside of ones own repository, the easiest way to depend
on Go commands is the following:

```sh
mkdir mydistro
cd mydistro
go mod init mydistro
```

Create a file with some unused build tag like this to create dependencies on
commands:

```go
//go:build tools

package something

import (
        _ "github.com/u-root/u-root/cmds/core/ip"
        _ "github.com/u-root/u-root/cmds/core/init"
        _ "github.com/hugelgupf/p9/cmd/p9ufs"
)
```

You can generate this file for your repo with the `gencmddeps` tool from
gobusybox:

```
go install github.com/u-root/gobusybox/src/cmd/gencmddeps@latest

gencmddeps -o deps.go -t tools -p something \
    github.com/u-root/u-root/cmds/core/{ip,init} \
    github.com/hugelgupf/p9/cmd/p9ufs
```

The unused build tag keeps it from being compiled, but its existence forces `go
mod tidy` to add these dependencies to `go.mod`:

```sh
go mod tidy

mkuimage \
  github.com/u-root/u-root/cmds/core/ip \
  github.com/u-root/u-root/cmds/core/init \
  github.com/hugelgupf/p9/cmd/p9ufs

# Works for offline vendored builds as well.
go mod vendor

mkuimage \
  github.com/u-root/u-root/cmds/core/ip \
  github.com/u-root/u-root/cmds/core/init \
  github.com/hugelgupf/p9/cmd/p9ufs
```

## Extra Files

You may also include additional files in the initramfs using the `-files` flag.

If you add binaries with `-files` are listed, their ldd dependencies will be
included as well.

```shell
$ mkuimage -files /bin/bash

$ cpio -ivt < /tmp/initramfs.linux_amd64.cpio
...
-rwxr-xr-x   0 root     root      1277936 Jan  1  1970 bin/bash
...
drwxr-xr-x   0 root     root            0 Jan  1  1970 lib/x86_64-linux-gnu
-rwxr-xr-x   0 root     root       210792 Jan  1  1970 lib/x86_64-linux-gnu/ld-linux-x86-64.so.2
-rwxr-xr-x   0 root     root      1926256 Jan  1  1970 lib/x86_64-linux-gnu/libc.so.6
lrwxrwxrwx   0 root     root           15 Jan  1  1970 lib/x86_64-linux-gnu/libtinfo.so.6 -> libtinfo.so.6.4
-rw-r--r--   0 root     root       216368 Jan  1  1970 lib/x86_64-linux-gnu/libtinfo.so.6.4
drwxr-xr-x   0 root     root            0 Jan  1  1970 lib64
lrwxrwxrwx   0 root     root           42 Jan  1  1970 lib64/ld-linux-x86-64.so.2 -> /lib/x86_64-linux-gnu/ld-linux-x86-64.so.2
...
```

You can determine placement with colons:

```shell
$ mkuimage -files "/bin/bash:sbin/sh"

$ cpio -ivt < /tmp/initramfs.linux_amd64.cpio
...
-rwxr-xr-x   0 root     root      1277936 Jan  1  1970 sbin/sh
...
```

For example on Debian, if you want to add two kernel modules for testing,
executing your currently booted kernel:

```shell
$ mkuimage -files "$HOME/hello.ko:etc/hello.ko" -files "$HOME/hello2.ko:etc/hello2.ko" ./u-root/cmds/core/*
$ qemu-system-x86_64 -kernel /boot/vmlinuz-$(uname -r) -initrd /tmp/initramfs.linux_amd64.cpio
```

## Cross Compilation (targeting different architectures and OSes)

Just like standard Go tooling, cross compilation is easy and supported.

To cross compile for an ARM, on Linux:

```shell
GOARCH=arm mkuimage ./u-root/cmds/core/*
```

If you are on OS X, and wish to build for Linux on AMD64:

```shell
GOOS=linux GOARCH=amd64 ./u-root/cmds/core/*
```

## Testing in QEMU

A good way to test the initramfs generated by u-root is with qemu:

```shell
qemu-system-x86_64 -nographic -kernel path/to/kernel -initrd /tmp/initramfs.linux_amd64.cpio
```

Note that you do not have to build a special kernel on your own, it is
sufficient to use an existing one. Usually you can find one in `/boot`.

If you don't have a kernel handy, you can also get the one we use for VM testing:

```shell
go install github.com/hugelgupf/vmtest/tools/runvmtest@latest

runvmtest -- bash -c "cp \$VMTEST_KERNEL ./kernel"
```

It may not have all features you require, however.

To automate testing, you may use the same
[vmtest](https://github.com/hugelgupf/vmtest) framework that we use as well. It
has native uimage support.

## Build Modes

mkuimage can create an initramfs in two different modes, specified by `-build`:

*   `bb` mode: One busybox-like binary comprising all the Go tools you ask to
    include.
    See [the gobusybox README for how it works](https://github.com/u-root/gobusybox).
    In this mode, mkuimage copies and rewrites the source of the tools you asked
    to include to be able to compile everything into one busybox-like binary.

*   `binary` mode: each specified binary is compiled separately and all binaries
    are added to the initramfs.
