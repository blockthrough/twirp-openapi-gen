package generator

import (
	"fmt"
	"github.com/emicklei/proto"
	"github.com/getkin/kin-openapi/openapi3"
	"path/filepath"
	"strings"
)

func (gen *generator) Handlers() []proto.Handler {
	return []proto.Handler{
		proto.WithPackage(gen.Package),
		proto.WithRPC(gen.RPC),
		proto.WithMessage(gen.Message),
		proto.WithImport(gen.Import),
	}
}

func (gen *generator) Package(pkg *proto.Package) {
	gen.packageName = pkg.Name
}

func (gen *generator) Import(i *proto.Import) {
	// the exclusion here is more about path traversal than it is
	// about the structure of google proto messages. The annotations
	// could serve to document a REST API, which goes beyond what
	// Twitch RPC does out of the box.
	if strings.Contains(i.Filename, "google/api/annotations.proto") {
		return
	}

	// TODO(dm): add mapping for google struct, wrappers, empty, and duration
	if strings.Contains(i.Filename, "google/protobuf") {
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
	parent, ok := rpc.Parent.(*proto.Service)
	if !ok {
		panic("parent is not proto.service")
	}

	pathName := filepath.Join("/"+gen.conf.pathPrefix+"/", gen.packageName+"."+parent.Name, rpc.Name)

	responseDesc := ""
	gen.openAPIV3.Paths[pathName] = &openapi3.PathItem{
		Post: &openapi3.Operation{
			Summary: rpc.Name,
			RequestBody: &openapi3.RequestBodyRef{
				Value: &openapi3.RequestBody{
					Content: openapi3.Content{"application/json": &openapi3.MediaType{
						Schema: &openapi3.SchemaRef{
							Ref: fmt.Sprintf("#/components/schemas/%s.%s", gen.packageName, rpc.RequestType),
						},
					}},
				},
			},
			Responses: map[string]*openapi3.ResponseRef{
				"200": {
					Value: &openapi3.Response{
						Description: &responseDesc,
						Content: openapi3.Content{"application/json": &openapi3.MediaType{
							Schema: &openapi3.SchemaRef{
								Ref: fmt.Sprintf("#/components/schemas/%s.%s", gen.packageName, rpc.ReturnsType),
							},
						},
						},
					},
				},
			},
		},
	}
}

func (gen *generator) Message(msg *proto.Message) {
	definitionName := fmt.Sprintf("%s.%s", gen.packageName, msg.Name)

	schemaPropsV3 := openapi3.Schemas{}

	fieldOrder := []string{}

	allFields := msg.Elements

	for _, element := range msg.Elements {
		switch val := element.(type) {
		case *proto.Oneof:
			// We're unpacking val.Elements into the field list,
			// which may or may not be correct. The oneof semantics
			// likely bring in edge-cases.
			allFields = append(allFields, val.Elements...)
		default:
			// No need to unpack for *proto.NormalField,...
		}
	}

	for _, element := range allFields {
		switch val := element.(type) {
		case *proto.Comment:
		case *proto.Oneof:
			// Nothing.
		case *proto.OneOfField:
			gen.addField(schemaPropsV3, &fieldOrder, val.Field, false)
		case *proto.MapField:
			gen.addField(schemaPropsV3, &fieldOrder, val.Field, false)
		case *proto.NormalField:
			gen.addField(schemaPropsV3, &fieldOrder, val.Field, val.Repeated)
		default:
			logger.log("unknown field type: %T", element)
		}
	}

	schemaDesc := description(msg.Comment)
	if len(fieldOrder) > 0 {
		// This is required to infer order, as json object keys
		// don't keep their order. Should have been an array.
		schemaDesc = schemaDesc + "\n\nFields: " + strings.Join(fieldOrder, ", ")
	}

	if gen.openAPIV3.Components.Schemas == nil {
		gen.openAPIV3.Components.Schemas = openapi3.Schemas{}
	}
	gen.openAPIV3.Components.Schemas[definitionName] = &openapi3.SchemaRef{
		Value: &openapi3.Schema{
			Title:       comment(msg.Comment),
			Description: strings.TrimSpace(schemaDesc),
			Type:        "object",
			Properties:  schemaPropsV3,
		},
	}
}

func (gen *generator) addField(schemaPropsV3 openapi3.Schemas, fieldOrder *[]string, field *proto.Field, repeated bool) {
	var allowedValues = []string{
		"boolean",
		"integer",
		"number",
		"object",
		"string",
	}

	fieldTitle := comment(field.Comment)
	fieldDescription := description(field.Comment)
	//fieldName := fmt.Sprintf("%s.%s", gen.packageName, field.Name)
	fieldName := field.Name
	fieldType := field.Type
	fieldFormat := field.Type

	p, ok := typeAliases[fieldType]
	if ok {
		fieldType = p.Type
		fieldFormat = p.Format
	}
	if fieldType == fieldFormat {
		fieldFormat = ""
	}

	*fieldOrder = append(*fieldOrder, fieldName)

	if _, ok := find(allowedValues, fieldType); ok {
		fieldSchemaV3 := openapi3.SchemaRef{
			Value: &openapi3.Schema{
				Title:       fieldTitle,
				Description: fieldDescription,
				Type:        fieldType,
				Format:      fieldFormat,
			},
		}
		if repeated {
			schemaPropsV3[fieldName] = &openapi3.SchemaRef{
				Value: &openapi3.Schema{
					Title:       fieldTitle,
					Description: fieldDescription,
					Type:        "array",
					Format:      fieldFormat,
					Items:       &fieldSchemaV3,
				},
			}
		} else {
			schemaPropsV3[fieldName] = &fieldSchemaV3
		}
		return
	}

	refV3 := fmt.Sprintf("#/components/schemas/%s", fieldType)

	if repeated {
		schemaPropsV3[fieldName] = &openapi3.SchemaRef{
			Value: &openapi3.Schema{
				Title:       fieldTitle,
				Description: fieldDescription,
				Type:        "array",
				Items: &openapi3.SchemaRef{
					Ref: refV3,
				},
			},
		}
		return
	}

	schemaPropsV3[fieldName] = &openapi3.SchemaRef{
		Ref: refV3,
		Value: &openapi3.Schema{
			Title:       fieldTitle,
			Description: fieldDescription,
		},
	}

	return
}

func comment(comment *proto.Comment) string {
	if comment == nil {
		return ""
	}

	result := ""
	for _, line := range comment.Lines {
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		result += " " + line
	}
	if len(result) > 1 {
		return result[1:]
	}
	return ""
}

func description(comment *proto.Comment) string {
	if comment == nil {
		return ""
	}

	grab := false

	result := []string{}
	for _, line := range comment.Lines {
		line = strings.TrimSpace(line)
		if line == "" {
			if grab {
				break
			}
			grab = true
			continue
		}
		if grab {
			result = append(result, line)
		}
	}
	return strings.Join(result, "\n")
}

func find(haystack []string, needle string) (int, bool) {
	for k, v := range haystack {
		if v == needle {
			return k, true
		}
	}
	return -1, false
}
