package main

import (
	"flag"
	"os"
	"strings"

	"github.com/apex/log"
	"github.com/davecgh/go-spew/spew"
	"github.com/diogogmt/twirp-openapi-gen/internal/generator"
)

var _ = spew.Dump

type arrayFlags []string

func (i *arrayFlags) String() string {
	return strings.Join(*i, ",")
}

func (i *arrayFlags) Set(value string) error {
	*i = append(*i, value)
	return nil
}

func main() {
	if err := run(os.Args); err != nil {
		log.WithError(err).Fatal("exit with error")
	}
}

func run(args []string) error {
	flags := flag.NewFlagSet(args[0], flag.ExitOnError)

	in := arrayFlags{}
	protoPaths := arrayFlags{}
	servers := arrayFlags{}
	flags.Var(&in, "in", "Input source .proto files. May be specified multiple times.")
	flags.Var(&protoPaths, "proto-path", "Specify the directory in which to search for imports. May be specified multiple times; directories will be searched in order.  If not given, the current working directory is used.")
	flags.Var(&servers, "servers", "Server object URL. May be specified multiple times.")
	title := flags.String("title", "open-api-v3-docs", "Document title")
	version := flags.String("version", "", "Document version")
	format := flags.String("format", "json", "Document format; json or yaml")
	out := flags.String("out", "./openapi-doc.json", "Output document file")
	pathPrefix := flags.String("path-prefix", "/twirp", "Twirp server path prefix")
	verbose := flags.Bool("verbose", false, "Log debug output")

	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	opts := []generator.Option{
		generator.ProtoPaths(protoPaths),
		generator.Servers(servers),
		generator.Title(*title),
		generator.Version(*version),
		generator.PathPrefix(*pathPrefix),
		generator.Format(*format),
		generator.Verbose(*verbose),
	}
	gen, err := generator.NewGenerator(in, *out, opts...)
	if err != nil {
		return err
	}
	if err := gen.Generate(); err != nil {
		return err
	}
	return nil
}
