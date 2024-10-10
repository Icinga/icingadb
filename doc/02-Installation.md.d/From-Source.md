# Installing Icinga DB from Source

## Using `go install`

You can build and install `icingadb` as follows:

```bash
go install github.com/icinga/icingadb@latest
```

This should place the `icingadb` binary in your configured `$GOBIN` path which defaults to `$GOPATH/bin` or
`$HOME/go/bin` if the `GOPATH` environment variable is not set.

## Build from Source

Download or clone the source and run the following command from the source's root directory.

```bash
go build -o icingadb cmd/icingadb/main.go
```

<!-- {% set from_source = True %} -->
<!-- {% include "02-Installation.md" %} -->
