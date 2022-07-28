# twirp-openapi-gen
Generate Open API V3 documentation for Twirp services

[![GoDoc](https://img.shields.io/badge/godoc-reference-5272B4.svg?style=for-the-badge)](https://godoc.org/github.com/diogogmt/twirp-openapi-gen)


## Installation

#### Binary

For installation instructions from binaries please visit the [Releases Page](https://diogogmt/twirp-openapi-gen/releases).

#### Go

```bash
$  go install github.com/diogogmt/twirp-openapi-gen/cmd/twirp-openapi-gen@latest
```

## Proto OpenAPI Mappings

| Proto                            | OpenAPI                               |
|----------------------------------|---------------------------------------|
| **RPC**                          | Path                                  |
| **Package + Service + RPC Name** | Path.Key                              |
| **RPC Name**                     | Path.Summary                          |
| **RPC Input**                    | Path.RequestBody                      |
| **RPC Output**                   | Path.Response                         |
| **RPC Comment**                  | Path.Description                      |
| **Message**                      | Component.Schema                      |
| **Message Comment**              | Component.Schema.Description          |
| **Message Field**                | Component.Schema.Property             |
| **Message Field Comment**        | Component.Schema.Property.Description |
| **Enum**                         | Component.Schema.Property.Enum        |


### Notes
* The requestBody property of the path post operation only has one content-type of application/json type, and its schema always references the RPC input message.
* Comments can be added above an RPC, message, or field resources. Inline comments are not supported.


## Usage

```bash
❯ twirp-openapi-gen -h
Usage of twirp-openapi-gen:
  -format string
        Document format; json or yaml (default "json")
  -in value
        Input source .proto files. May be specified multiple times.
  -out string
        Output document file (default "./openapi-doc.json")
  -path-prefix string
        Twirp server path prefix (default "/twirp")
  -proto-path value
        Specify the directory in which to search for imports. May be specified multiple times; directories will be searched in order.  If not given, the current working directory is used.
  -servers value
        Server object URL. May be specified multiple times.
  -title string
        Document title (default "open-api-v3-docs")
  -verbose
        Log debug output
  -version string
        Document version
```

### Examples

```bash
```

## Contributing

#### Makefile

```bash

```

## Why

This project is a rewrite of the [twirp-swagger-gen](https://github.com/go-bridget/twirp-swagger-gen) tool adding support to the latest [OpenAPI v3 spec](https://github.com/OAI/OpenAPI-Specification/blob/main/versions/3.0.3.md)

Even though there are other ways of generating documentation for proto files, eg; [protoc-gen-doc](github.com/pseudomuto/protoc-gen-doc/), [gnostic](https://github.com/google/gnostic), buf modules, etc. Neither provides an out-of the box solution to have interactive Twirp API docs. 

There is already a rich ecosystem of tools for visualizing and interacting with OpenAPI V3 spec documents. The twirp-openapi-gen tool leverages that and generates valid JSON/YAML OpenAPI V3 documents from the proto definitions.
The API docs can be imported to any tool that has support for OpenAPI V3, eg; Postman, Swagger, Stoplight, etc..
