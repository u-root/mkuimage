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
git clone https://github.com/u-root/cpu
```

Build gobusybox binaries of these two commands and add to initramfs:

```shell
$ mkuimage ./u-root/cmds/core/{init,gosh}

$ cpio -ivt < /tmp/initramfs.linux_amd64.cpio
...
-rwxr-x---   0 root     root      4542464 Jan  1  1970 bbin/bb
lrwxrwxrwx   0 root     root            2 Jan  1  1970 bbin/gosh -> bb
lrwxrwxrwx   0 root     root            2 Jan  1  1970 bbin/init -> bb
...
```

Add symlinks for shell and init:

```shell
$ mkuimage -initcmd=init -defaultsh=gosh ./u-root/cmds/core/{init,gosh}

$ cpio -ivt < /tmp/initramfs.linux_amd64.cpio
...
lrwxrwxrwx   0 root     root           12 Jan  1  1970 bin/defaultsh -> ../bbin/gosh
lrwxrwxrwx   0 root     root           12 Jan  1  1970 bin/sh -> ../bbin/gosh
...
lrwxrwxrwx   0 root     root            9 Jan  1  1970 init -> bbin/init
...
```

Build everything from core without ls and losetup:

```shell
$ mkuimage ./u-root/cmds/core/* -./u-root/cmds/core/{ls,losetup}
```

Build an initramfs with init, gosh and cpud in a gobusybox binary:

> [!IMPORTANT]
> Since the commands are in 2 different modules, and cpud has a dependency on
> u-root, we can't build one binary without resolving the dependency issue.
>
> Since no go.mod is found in the current directory, one will be synthesized as
> the warning advertises.
>
> To properly resolve these dependencies, head down to the [multi-module uimages section](#multi-module-uimages).

```shell
$ mkuimage ./u-root/cmds/core/{init,gosh} ./cpu/cmds/cpud
...
01:24:15 INFO GBB_STRICT is not set.
01:24:15 INFO [WARNING] github.com/u-root/cpu/cmds/cpud depends on github.com/u-root/u-root @ version v0.11.1-0.20230913033713-004977728a9d
01:24:15 INFO   Using github.com/u-root/u-root @ directory /home/u-root to build it.
...

$ cpio -ivt < /tmp/initramfs.linux_amd64.cpio
...
-rwxr-x---   0 root     root      6365184 Jan  1  1970 bbin/bb
lrwxrwxrwx   0 root     root            2 Jan  1  1970 bbin/cpud -> bb
lrwxrwxrwx   0 root     root            2 Jan  1  1970 bbin/gosh -> bb
lrwxrwxrwx   0 root     root            2 Jan  1  1970 bbin/init -> bb
...
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

To cross compile for an ARM, on Linux:

```shell
GOARCH=arm mkuimage ./u-root/cmds/core/*
```

If you are on OSX, and wish to build for Linux on AMD64:

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

## Multi-module uimages

Rather than having mkuimage decide how to resolve dependencies across
multi-module repositories, you may also create a go.mod with all commands you
intend to use in them.

To depend on commands outside of ones own repository, the easiest way to depend
on Go commands is the following:

```sh
TMPDIR=$(mktemp -d)
cd $TMPDIR
go mod init foobar
```

Create a file with some unused build tag like this to create dependencies on
commands:

```go
//go:build tools

package something

import (
        "github.com/u-root/u-root/cmds/core/ip"
        "github.com/u-root/u-root/cmds/core/init"
        "github.com/hugelgupf/p9/cmd/p9ufs"
)
```

The unused build tag keeps it from being compiled, but its existence forces `go
mod tidy` to add these dependencies to `go.mod`:

```sh
go mod tidy

mkuimage \
  github.com/u-root/u-root/cmds/core/ip \
  github.com/u-root/u-root/cmds/core/init \
  github.com/hugelgupf/p9/cmd/p9ufs
```

## Build Modes

mkuimage can create an initramfs in two different modes, specified by `-build`:

*   `bb` mode: One busybox-like binary comprising all the Go tools you ask to
    include.
    See [the gobusybox README for how it works](https://github.com/u-root/gobusybox).
    In this mode, mkuimage copies and rewrites the source of the tools you asked
    to include to be able to compile everything into one busybox-like binary.

*   `binary` mode: each specified binary is compiled separately and all binaries
    are added to the initramfs.
