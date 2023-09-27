package generator

import (
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/emicklei/proto"
	"github.com/getkin/kin-openapi/openapi3"
)

const (
	googleAnyType       = "google.protobuf.Any"
	googleListValueType = "google.protobuf.ListValue"
)

var (
	successDescription = "Success"
)

func (gen *generator) Handlers() []proto.Handler {
	return []proto.Handler{
		proto.WithPackage(gen.Package),
		proto.WithImport(gen.Import),
		proto.WithRPC(gen.RPC),
		proto.WithEnum(gen.Enum),
		proto.WithMessage(gen.Message),
	}
}

func (gen *generator) Package(pkg *proto.Package) {
	logger.logd("Package handler %q", pkg.Name)
	gen.packageName = pkg.Name
}

func (gen *generator) Import(i *proto.Import) {
	logger.logd("Import handler %q %q", gen.packageName, i.Filename)

	if _, ok := gen.importedFiles[i.Filename]; ok {
		return
	}
	gen.importedFiles[i.Filename] = struct{}{}

	// Instead of loading and generating the OpenAPI docs for the google proto definitions,
	// its known types are mapped to OpenAPI types; see aliases.go.
	if strings.Contains(i.Filename, "google/") {
		return
	}

	protoFile, err := readProtoFile(i.Filename, gen.conf.protoPaths)
	if err != nil {
		logger.log("could not import file %q", i.Filename)
		return
	}

	oldPackageName := gen.packageName

	// Override the package name for the next round of Walk calls to preserve the types full import path
	withPackage := func(pkg *proto.Package) {
		gen.packageName = pkg.Name
	}

	// additional files walked for messages and imports only
	proto.Walk(protoFile,
		proto.WithPackage(withPackage),
		proto.WithImport(gen.Import),
		proto.WithRPC(gen.RPC),
		proto.WithEnum(gen.Enum),
		proto.WithMessage(gen.Message),
	)

	gen.packageName = oldPackageName
}

func (gen *generator) RPC(rpc *proto.RPC) {
	logger.logd("RPC handler %q %q %q %q", gen.packageName, rpc.Name, rpc.RequestType, rpc.ReturnsType)

	parent, ok := rpc.Parent.(*proto.Service)
	if !ok {
		log.Panicf("parent is not proto.service")
	}
	pathName := filepath.Join("/"+gen.conf.pathPrefix+"/", gen.packageName+"."+parent.Name, rpc.Name)

	var reqMediaType *openapi3.MediaType
	switch rpc.RequestType {
	case "google.protobuf.Empty":
		reqMediaType = openapi3.NewMediaType()
	default:
		reqMediaType = &openapi3.MediaType{
			Schema: &openapi3.SchemaRef{
				Ref: fmt.Sprintf("#/components/schemas/%s.%s", gen.packageName, rpc.RequestType),
			},
		}
	}

	var resMediaType *openapi3.MediaType
	switch rpc.ReturnsType {
	case "google.protobuf.Empty":
		resMediaType = openapi3.NewMediaType()
	default:
		resMediaType = &openapi3.MediaType{
			Schema: &openapi3.SchemaRef{
				Ref: fmt.Sprintf("#/components/schemas/%s.%s", gen.packageName, rpc.ReturnsType),
			},
		}
	}

	_, reqExamples, resExamples, err := parseComment(rpc.Comment)
	if err != nil {
		// TODO(dm): how can we surface the errors from the parser instead of panicking?
		log.Panicf("failed to parse comment %s ", err)
	}
	reqMediaType.Examples = map[string]*openapi3.ExampleRef{}
	for i, example := range reqExamples {
		reqMediaType.Examples[strconv.FormatInt(int64(i), 10)] = &openapi3.ExampleRef{
			Value: &openapi3.Example{
				Summary: fmt.Sprintf("example %d", i),
				Value:   example,
			},
		}
	}
	resMediaType.Examples = map[string]*openapi3.ExampleRef{}
	for i, example := range resExamples {
		resMediaType.Examples[strconv.FormatInt(int64(i), 10)] = &openapi3.ExampleRef{
			Value: &openapi3.Example{
				Summary: fmt.Sprintf("example %d", i),
				Value:   example,
			},
		}
	}
	gen.openAPIV3.Paths[pathName] = &openapi3.PathItem{
		Description: description(rpc.Comment),
		Post: &openapi3.Operation{
			Summary: rpc.Name,
			RequestBody: &openapi3.RequestBodyRef{
				Value: &openapi3.RequestBody{
					Content: openapi3.Content{"application/json": reqMediaType},
				},
			},
			Responses: map[string]*openapi3.ResponseRef{
				"200": {
					Value: &openapi3.Response{
						Description: &successDescription,
						Content:     openapi3.Content{"application/json": resMediaType},
					},
				},
			},
		},
	}
}

func (gen *generator) Enum(enum *proto.Enum) {
	logger.logd("Enum handler %q %q", gen.packageName, enum.Name)
	values := []interface{}{}
	for _, element := range enum.Elements {
		enumField := element.(*proto.EnumField)
		values = append(values, enumField.Name)
	}

	gen.openAPIV3.Components.Schemas[gen.packageName+"."+enum.Name] = &openapi3.SchemaRef{
		Value: &openapi3.Schema{
			Description: description(enum.Comment),
			Type:        "string",
			Enum:        values,
		},
	}
}

func (gen *generator) Message(msg *proto.Message) {
	logger.logd("Message handler %q %q", gen.packageName, msg.Name)

	schemaProps := openapi3.Schemas{}

	for _, element := range msg.Elements {
		switch val := element.(type) {
		case *proto.Message:
			//logger.logd("proto.Message")
			gen.Message(val)
		case *proto.Comment:
			//logger.logd("proto.Comment")
		case *proto.Oneof:
			//logger.logd("proto.Oneof")
		case *proto.OneOfField:
			//logger.logd("proto.OneOfField")
			gen.addField(schemaProps, val.Field, false)
		case *proto.MapField:
			//logger.logd("proto.MapField")
			gen.addField(schemaProps, val.Field, false)
		case *proto.NormalField:
			//logger.logd("proto.NormalField %q %q", val.Field.Type, val.Field.Name)
			gen.addField(schemaProps, val.Field, val.Repeated)
		default:
			logger.logd("unknown field type: %T", element)
		}
	}

	gen.openAPIV3.Components.Schemas[gen.packageName+"."+msg.Name] = &openapi3.SchemaRef{
		Value: &openapi3.Schema{
			Description: description(msg.Comment),
			Type:        "object",
			Properties:  schemaProps,
		},
	}
}

func (gen *generator) addField(schemaPropsV3 openapi3.Schemas, field *proto.Field, repeated bool) {
	fieldDescription := description(field.Comment)
	fieldName := field.Name
	fieldType := field.Type
	fieldFormat := field.Type
	// map proto types to openapi
	if p, ok := typeAliases[fieldType]; ok {
		fieldType = p.Type
		fieldFormat = p.Format
	}

	if fieldType == fieldFormat {
		fieldFormat = ""
	}

	switch fieldType {
	// Build the schema for native types that don't need to reference other schemas
	// https://github.com/OAI/OpenAPI-Specification/blob/main/versions/3.0.3.md#data-types
	case "boolean", "integer", "number", "string", "object":
		fieldSchemaV3 := openapi3.SchemaRef{
			Value: &openapi3.Schema{
				Description: fieldDescription,
				Type:        fieldType,
				Format:      fieldFormat,
			},
		}
		if !repeated {
			schemaPropsV3[fieldName] = &fieldSchemaV3
			return
		}
		schemaPropsV3[fieldName] = &openapi3.SchemaRef{
			Value: &openapi3.Schema{
				Description: fieldDescription,
				Type:        "array",
				Format:      fieldFormat,
				Items:       &fieldSchemaV3,
			},
		}
		return

	// generate the schema for google well known complex types: https://protobuf.dev/reference/protobuf/google.protobuf/#index
	case "google.protobuf.Any":
		logger.logd("Any - %s type:%q, format:%q", fieldName, fieldType, fieldFormat)
		gen.addGoogleAnySchema()
	case "google.protobuf.ListValue":
		logger.logd("ListValue - %s type:%q, format:%q", fieldName, fieldType, fieldFormat)
		gen.addGoogleListValueSchema()
	default:
		logger.logd("DEFAULT %s type:%q, format:%q", fieldName, fieldType, fieldFormat)
	}

	// prefix custom types with the package name
	ref := fmt.Sprintf("#/components/schemas/%s", fieldType)
	if !strings.Contains(fieldType, ".") {
		ref = fmt.Sprintf("#/components/schemas/%s.%s", gen.packageName, fieldType)
	}

	if !repeated {
		schemaPropsV3[fieldName] = &openapi3.SchemaRef{
			Ref: ref,
			Value: &openapi3.Schema{
				Description: fieldDescription,
				Type:        "object",
			},
		}
		return
	}

	schemaPropsV3[fieldName] = &openapi3.SchemaRef{
		Value: &openapi3.Schema{
			Description: fieldDescription,
			Type:        "array",
			Items: &openapi3.SchemaRef{
				Ref: ref,
				Value: &openapi3.Schema{
					Type: "object",
				},
			},
		},
	}
}

// addGoogleAnySchema adds a schema item for the google.protobuf.Any type.
func (gen *generator) addGoogleAnySchema() {
	if _, ok := gen.openAPIV3.Components.Schemas[googleAnyType]; ok {
		return
	}
	gen.openAPIV3.Components.Schemas[googleAnyType] = &openapi3.SchemaRef{
		Value: &openapi3.Schema{
			Description: `
The JSON representation of an Any value uses the regular
representation of the deserialized, embedded message, with an
additional field @type which contains the type URL. Example:

	package google.profile;
	message Person {
	  string first_name = 1;
	  string last_name = 2;
	}

	{
	  "@type": "type.googleapis.com/google.profile.Person",
	  "firstName": <string>,
	  "lastName": <string>
	}

If the embedded message type is well-known and has a custom JSON
representation, that representation will be embedded adding a field
value which holds the custom JSON in addition to the @type
field. Example (for message [google.protobuf.Duration][]):

	{
	  "@type": "type.googleapis.com/google.protobuf.Duration",
	  "value": "1.212s"
	}
`,
			Type: "object",
			Properties: openapi3.Schemas{
				"@type": &openapi3.SchemaRef{
					Value: &openapi3.Schema{
						Description: "",
						Type:        "string",
						Format:      "",
					},
				},
			},
		},
	}
}

// addGoogleAnySchema adds a schema item for the google.protobuf.ListValue type.
func (gen *generator) addGoogleListValueSchema() {
	if _, ok := gen.openAPIV3.Components.Schemas[googleListValueType]; ok {
		return
	}
	gen.openAPIV3.Components.Schemas[googleListValueType] = &openapi3.SchemaRef{
		Value: &openapi3.Schema{
			Description: `
ListValue is a wrapper around a repeated field of values.
The JSON representation for ListValue is JSON array.
`,
			Type: "array",
			Items: &openapi3.SchemaRef{
				Value: &openapi3.Schema{
					OneOf: openapi3.SchemaRefs{
						&openapi3.SchemaRef{
							Value: &openapi3.Schema{
								Type: "string",
							},
						},
						&openapi3.SchemaRef{
							Value: &openapi3.Schema{
								Type: "number",
							},
						},
						&openapi3.SchemaRef{
							Value: &openapi3.Schema{
								Type: "integer",
							},
						},
						&openapi3.SchemaRef{
							Value: &openapi3.Schema{
								Type: "bool",
							},
						},
						&openapi3.SchemaRef{
							Value: &openapi3.Schema{
								Type: "array",
							},
						},
						&openapi3.SchemaRef{
							Value: &openapi3.Schema{
								Type: "object",
							},
						},
					},
				},
			},
		},
	}
}

func description(comment *proto.Comment) string {
	if comment == nil {
		return ""
	}
	result := []string{}
	for _, line := range comment.Lines {
		line = strings.TrimSpace(line)
		if len(line) > 0 {
			result = append(result, line)
		}
	}
	return strings.Join(result, "\n")
}

// parseComment parses the comment for an RPC method and returns the description, request examples, and response examples.
// it looks for the labels req-example: and res-example: to extract the JSON payload samples.
func parseComment(comment *proto.Comment) (string, []map[string]interface{}, []map[string]interface{}, error) {
	if comment == nil {
		return "", nil, nil, nil
	}
	reqExamples := []map[string]interface{}{}
	respExamples := []map[string]interface{}{}
	message := ""
	for _, line := range comment.Lines {
		line = strings.TrimLeft(line, " ")
		if strings.HasPrefix(line, "req-example:") {
			parts := strings.Split(line, "req-example:")
			example := map[string]interface{}{}
			if err := json.Unmarshal([]byte(parts[1]), &example); err != nil {
				return "", nil, nil, err
			}
			reqExamples = append(reqExamples, example)
		} else if strings.HasPrefix(line, "res-example:") {
			parts := strings.Split(line, "res-example:")
			example := map[string]interface{}{}
			if err := json.Unmarshal([]byte(parts[1]), &example); err != nil {
				return "", nil, nil, err
			}
			respExamples = append(respExamples, example)
		} else {
			message = fmt.Sprintf("%s\n%s", message, line)
		}
	}
	return message, reqExamples, respExamples, nil
}
