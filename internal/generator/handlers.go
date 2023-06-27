package generator

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/emicklei/proto"
	"github.com/getkin/kin-openapi/openapi3"
)

var (
	successDescription = "Success"
)

func (gen *generator) Handlers() []proto.Handler {
	return []proto.Handler{
		proto.WithPackage(gen.Package),
		proto.WithRPC(gen.RPC),
		proto.WithMessage(gen.Message),
		proto.WithImport(gen.Import),
		proto.WithEnum(gen.Enum),
	}
}

func (gen *generator) Enum(enum *proto.Enum) {
	logger.logd("Enum handler %q %q", gen.packageName, enum.Name)
	values := []string{}
	for _, element := range enum.Elements {
		enumField := element.(*proto.EnumField)
		values = append(values, enumField.Name)
	}
	gen.enums[gen.packageName+"."+enum.Name] = values
}

func (gen *generator) Package(pkg *proto.Package) {
	logger.logd("Package handler %q", pkg.Name)
	gen.packageName = pkg.Name
}

func (gen *generator) Import(i *proto.Import) {
	logger.logd("Import handler %q %q", gen.packageName, i.Filename)

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

	withPackage := func(pkg *proto.Package) {
		gen.packageName = pkg.Name
	}

	// additional files walked for messages and imports only
	proto.Walk(protoFile, proto.WithPackage(withPackage), proto.WithImport(gen.Import), proto.WithMessage(gen.Message))

	gen.packageName = oldPackageName
}

func (gen *generator) RPC(rpc *proto.RPC) {
	logger.logd("RPC handler %q %q %q %q", gen.packageName, rpc.Name, rpc.RequestType, rpc.ReturnsType)
	parent, ok := rpc.Parent.(*proto.Service)
	if !ok {
		panic("parent is not proto.service")
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

func (gen *generator) Message(msg *proto.Message) {
	logger.logd("Message handler %q %q", gen.packageName, msg.Name)

	schemaProps := openapi3.Schemas{}

	// TODO(dm): test OneOf elements
	//for _, element := range msg.Elements {
	//	switch val := element.(type) {
	//	case *proto.Oneof:
	//		// We're unpacking val.Elements into the field list,
	//		// which may or may not be correct. The oneof semantics
	//		// likely bring in edge-cases.
	//		allFields = append(allFields, val.Elements...)
	//	default:
	//		// No need to unpack for *proto.NormalField,...
	//	}
	//}

	for _, element := range msg.Elements {
		switch val := element.(type) {
		case *proto.Message:
			logger.logd("proto.Message")
			gen.Message(val)
		case *proto.Comment:
			logger.logd("proto.Comment")
		case *proto.Oneof:
			logger.logd("proto.Oneof")
		case *proto.OneOfField:
			logger.logd("proto.OneOfField")
			gen.addField(schemaProps, val.Field, false)
		case *proto.MapField:
			logger.logd("proto.MapField")
			gen.addField(schemaProps, val.Field, false)
		case *proto.NormalField:
			logger.logd("proto.NormalField")
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
	// map proto types to open api
	if p, ok := typeAliases[fieldType]; ok {
		fieldType = p.Type
		fieldFormat = p.Format
	}

	if fieldType == fieldFormat {
		fieldFormat = ""
	}

	// Build the schema for native types that don't need to reference other schemas
	// https://github.com/OAI/OpenAPI-Specification/blob/main/versions/3.0.3.md#data-types
	switch fieldType {
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
	}

	if enumValues, ok := gen.enums[gen.packageName+"."+field.Type]; ok {
		enumI := []interface{}{}
		for _, v := range enumValues {
			enumI = append(enumI, v)
		}
		if !repeated {
			schemaPropsV3[fieldName] = &openapi3.SchemaRef{
				Value: &openapi3.Schema{
					Type:        "string",
					Enum:        enumI,
					Description: fieldDescription,
				},
			}
			return
		}
		schemaPropsV3[fieldName] = &openapi3.SchemaRef{
			Value: &openapi3.Schema{
				Description: fieldDescription,
				Type:        "array",
				Items: &openapi3.SchemaRef{
					Value: &openapi3.Schema{
						Type: "string",
						Enum: enumI,
					},
				},
			},
		}
		return
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
