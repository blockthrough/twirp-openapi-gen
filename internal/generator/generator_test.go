package generator

import (
	"testing"
)

type ProtoRPC struct {
	name   string
	input  string
	output string
	desc   string
}

type ProtoMessage struct {
	name   string
	fields []ProtoField
}

type ProtoField struct {
	name      string
	fieldType string
	format    string
	desc      string
	enums     []string
	ref       string
	itemsRef  string
	itemsType string
}

func TestGenerator(t *testing.T) {
	opts := []Option{
		ProtoPaths([]string{"./testdata/paymentapis", "./testdata/petapis"}),
		Servers([]string{"https://example.com"}),
		Title("Test"),
		DocVersion("0.1"),
		Format("json"),
		Verbose(true),
	}
	gen, err := NewGenerator([]string{"./testdata/petapis/pet/v1/pet.proto"}, opts...)
	if err != nil {
		t.Fatal(err)
	}
	openAPI, err := gen.Parse()
	if err != nil {
		t.Fatal(err)
	}

	if err := gen.Save("./testdata/doc.json"); err != nil {
		t.Fatal(err)
	}

	pkgName := "pet.v1"
	serviceName := "PetStoreService"
	rpcs := []ProtoRPC{
		{
			name:   "GetPet",
			input:  "GetPetRequest",
			output: "GetPetResponse",
			desc:   "GetPet returns details about a pet\nIt accepts a pet id as an input and returns back the matching pet object",
		},
	}
	messages := []ProtoMessage{
		{
			name: "GetPetRequest",
			fields: []ProtoField{
				{
					name:      "pet_id",
					fieldType: "string",
				},
			},
		},
		{
			name: "Pet",
			fields: []ProtoField{
				{
					name:      "pet_type",
					fieldType: "object",
					// TODO(dm): check if the enum values were added to the schema
					ref: "#/components/schemas/pet.v1.PetType",
				},
				{
					name:      "pet_types",
					fieldType: "array",
					itemsType: "#/components/schemas/pet.v1.PetType",
				},
				{
					name:      "pet_id",
					fieldType: "string",
					desc:      "pet_id is an auto-generated id for the pet\\nthe id uniquely identifies a pet in the system",
				},
				{
					name:      "name",
					fieldType: "string",
				},
				{
					name:      "created_at",
					fieldType: "string",
					format:    "date-time",
				},
				{
					name:      "vet",
					fieldType: "object",
					ref:       "#/components/schemas/pet.v1.Vet",
				},
				{
					name:      "vets",
					fieldType: "array",
					itemsRef:  "#/components/schemas/pet.v1.Vet",
					itemsType: "object",
				},
			},
		},
	}

	t.Run("RPC", func(t *testing.T) {
		for _, rpc := range rpcs {
			pathName := "/" + pkgName + "." + serviceName + "/" + rpc.name
			path, ok := openAPI.Paths[pathName]
			if !ok {
				t.Errorf("%s: missing rpc %q", pathName, rpc.name)
			}

			if path.Description != rpc.desc {
				t.Errorf("%s: expected desc %q but got %q", pathName, rpc.desc, path.Description)
			}

			post := path.Post
			if post == nil {
				t.Errorf("%s: missing post", pathName)
				continue
			}

			if post.Summary != rpc.name {
				t.Errorf("%s: expected summary %q but got %q", pathName, rpc.name, post.Summary)
			}

			requestBodyRef := post.RequestBody
			if requestBodyRef == nil {
				t.Errorf("%s: missing request body", pathName)
				continue
			}

			// request
			{
				requestBody := requestBodyRef.Value
				if requestBody == nil {
					t.Errorf("%s: missing request body", pathName)
					continue
				}

				mediaType, ok := requestBody.Content["application/json"]
				if !ok {
					t.Errorf("%s: missing content type", pathName)
					continue
				}

				if mediaType.Schema == nil {
					t.Errorf("%s: missing media type schema", pathName)
					continue
				}

				expectedRef := "#/components/schemas/" + pkgName + "." + rpc.input
				if mediaType.Schema.Ref != expectedRef {
					t.Errorf("%s: expected ref %q but got %q", pathName, expectedRef, mediaType.Schema.Ref)
				}
			}

			// response
			{
				respRef := post.Responses["200"]
				if respRef == nil {
					t.Errorf("%s: missing resp", pathName)
					continue
				}

				resp := respRef.Value
				if resp == nil {
					t.Errorf("%s: missing resp", pathName)
					continue
				}

				mediaType, ok := resp.Content["application/json"]
				if !ok {
					t.Errorf("%s: missing content type", pathName)
					continue
				}

				if mediaType.Schema == nil {
					t.Errorf("%s: missing media type schema", pathName)
					continue
				}

				expectedRef := "#/components/schemas/" + pkgName + "." + rpc.output
				if mediaType.Schema.Ref != expectedRef {
					t.Errorf("%s: expected ref %q but got %q", pathName, expectedRef, mediaType.Schema.Ref)
				}
			}
		}
	})

	t.Run("Messages", func(*testing.T) {
		for _, message := range messages {
			schemaName := "" + pkgName + "." + message.name
			schema, ok := openAPI.Components.Schemas[schemaName]
			if !ok {
				t.Errorf("%s: missing message %q", schemaName, message.name)
			}
			if schema.Value == nil {
				t.Errorf("%s: missing component", schemaName)
				continue
			}
			properties := schema.Value.Properties
			for _, messageField := range message.fields {
				propertyRef, ok := properties[messageField.name]
				if !ok {
					t.Errorf("%s: missing property %q", schemaName, messageField.name)
				}

				if propertyRef == nil || propertyRef.Value == nil {
					t.Errorf("%s: missing property ref", schemaName)
					continue
				}

				property := propertyRef.Value
				if property.Type != messageField.fieldType {
					t.Errorf("%s: %q expected property type %q but got %q", schemaName, message.name, messageField.fieldType, property.Type)
					continue
				}

				if messageField.format != "" {
					if messageField.format != "" && property.Format != messageField.format {
						t.Errorf("%s: expected property format %q but got %q", schemaName, messageField.format, property.Format)
						continue
					}
				}

				if propertyRef.Ref != messageField.ref {
					t.Errorf("%s: %q expected reference %q but got %q", schemaName, messageField.name, messageField.ref, propertyRef.Ref)
				}

				// TODO(dm): update test to deference an enum schema instead of an array
				//enums := map[string]struct{}{}
				//if property.Type == "array" {
				//	if property.Items == nil || property.Items.Value == nil {
				//		t.Errorf("%s: missing property enum array items", schemaName)
				//	}
				//	for _, enum := range property.Items.Value.Enum {
				//		enums[enum.(string)] = struct{}{}
				//	}
				//
				//	if property.Items.Value.Type != messageField.itemsType {
				//		t.Errorf("%s: expected %s items type %q but got %q", schemaName, messageField.name, messageField.itemsType, property.Items.Value.Type)
				//	}
				//	if property.Items.Ref != messageField.itemsRef {
				//		t.Errorf("%s: expected %s items ref %q but got %q", schemaName, messageField.name, messageField.itemsRef, property.Items.Ref)
				//	}
				//}
				//
				//for _, enum := range property.Enum {
				//	enums[enum.(string)] = struct{}{}
				//}
				//for _, enum := range messageField.enums {
				//	if _, ok := enums[enum]; !ok {
				//		t.Errorf("%s: %s missing enum %q", schemaName, messageField.name, enum)
				//	}
				//}
			}
		}
	})
}
