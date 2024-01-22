#!/bin/bash

set -eu

GO="go"
if [ -v GOROOT ];
then
  GO="$GOROOT/bin/go"
fi

function buildem() {
  for GOOS in $1
  do
    for GOARCH in $2
    do
      echo "Building $GOOS/$GOARCH..."
      GOOS=$GOOS GOARCH=$GOARCH $GO build ./...
    done
  done
}

GOARCHES="386 amd64 arm arm64 ppc64 ppc64le s390x mips mipsle mips64 mips64le"
buildem "linux" "$GOARCHES"

GOARCHES="386 amd64 arm arm64"
GOOSES="freebsd" # TBD netbsd openbsd windows
buildem "$GOOSES" "$GOARCHES"

GOARCHES_DARWIN="arm64 amd64"
buildem "darwin" "$GOARCHES_DARWIN"
