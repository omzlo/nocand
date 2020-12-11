# nocand

This repository contains the source code for `nocand` the NoCAN server.

For more information see http://omzlo.com/the-nocan-platform

This code is licensed under the MIT license as described in LICENSE.

## Building nocand

The `nocand` tool is written in [GO](https://golang.org/). You will need to download and install GO if you want to build `nocand` yourself instead of using the official binaries.

Downloading all the source code and all dependencies is as simple as:

```
git clone https://github.com/omzlo/nocand.git
cd nocand
go get ./...
```

Building the executable:

```
go build cmd/nocand.go
```
