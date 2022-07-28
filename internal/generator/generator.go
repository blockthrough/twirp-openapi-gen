package generator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/emicklei/proto"
	"github.com/getkin/kin-openapi/openapi3"
)

var logger Lg

type generatorConfig struct {
	protoPaths []string
	servers    []string
	title      string
	version    string
	pathPrefix string
	format     string
	verbose    bool
}

type Option func(config *generatorConfig) error

func ProtoPaths(paths []string) Option {
	return func(config *generatorConfig) error {
		config.protoPaths = paths
		return nil
	}
}

func Servers(servers []string) Option {
	return func(config *generatorConfig) error {
		config.servers = servers
		return nil
	}
}

func Title(title string) Option {
	return func(config *generatorConfig) error {
		config.title = title
		return nil
	}
}

func Version(version string) Option {
	return func(config *generatorConfig) error {
		config.version = version
		return nil
	}
}

func PathPrefix(pathPrefix string) Option {
	return func(config *generatorConfig) error {
		config.pathPrefix = pathPrefix
		return nil
	}
}

func Format(format string) Option {
	return func(config *generatorConfig) error {
		config.format = format
		return nil
	}
}

func Verbose(verbose bool) Option {
	return func(config *generatorConfig) error {
		logger.verbose = verbose
		return nil
	}
}

type generator struct {
	openAPIV3 *openapi3.T

	conf        *generatorConfig
	inputFiles  []string
	packageName string

	enums map[string][]string
}

func NewGenerator(inputFiles []string, options ...Option) (*generator, error) {
	conf := generatorConfig{}
	for _, opt := range options {
		if err := opt(&conf); err != nil {
			return nil, err
		}
	}

	if len(inputFiles) < 1 {
		return nil, fmt.Errorf("missing input files")
	}

	openAPIV3 := openapi3.T{
		OpenAPI: "3.0.0",
		Info: &openapi3.Info{
			Title:   conf.title,
			Version: conf.version,
		},
		Paths: openapi3.Paths{},
		Components: openapi3.Components{
			Schemas: map[string]*openapi3.SchemaRef{},
		},
	}

	for _, server := range conf.servers {
		openAPIV3.Servers = append(openAPIV3.Servers, &openapi3.Server{URL: server})
	}

	logger.logd("generating %q doc for %v", conf.format, inputFiles)

	return &generator{
		inputFiles: inputFiles,
		openAPIV3:  &openAPIV3,
		conf:       &conf,
		enums:      map[string][]string{},
	}, nil
}

func (gen *generator) Generate(filename string) error {
	if _, err := gen.Parse(); err != nil {
		return err
	}

	if err := gen.Save(filename); err != nil {
		return err
	}

	return nil
}

func (gen *generator) Parse() (*openapi3.T, error) {
	for _, filename := range gen.inputFiles {
		protoFile, err := readProtoFile(filename, gen.conf.protoPaths)
		if err != nil {
			return nil, fmt.Errorf("readProtoFile: %w", err)
		}
		proto.Walk(protoFile, gen.Handlers()...)
	}

	logger.logd("generated %d path(s) and %d component(s)", len(gen.openAPIV3.Paths), len(gen.openAPIV3.Components.Schemas))
	return gen.openAPIV3, nil
}

func (gen *generator) Save(filename string) error {
	var by []byte
	var err error
	switch gen.conf.format {
	case "json":
		by, err = gen.JSON()
	case "yaml", "yml":
		return fmt.Errorf("%q format not supported", gen.conf.format)
	default:
		return fmt.Errorf("missing format")
	}
	if err != nil {
		return err
	}

	return os.WriteFile(filename, by, os.ModePerm^0111)
}

func (gen *generator) JSON() ([]byte, error) {
	return json.MarshalIndent(gen.openAPIV3, "", "  ")
}

func readProtoFile(filename string, protoPaths []string) (*proto.Proto, error) {
	var file *os.File
	var err error
	for _, path := range append(protoPaths, "") {
		file, err = os.Open(filepath.Join(path, filename))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("Open: %w", err)
		}
		break
	}
	if file == nil {
		return nil, fmt.Errorf("could not read file %q", filename)
	}
	defer file.Close()

	parser := proto.NewParser(file)
	return parser.Parse()
}
