package protocol

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/ThinkInAIXYZ/go-mcp/pkg"
)

type DataType string

const (
	ObjectT DataType = "object"
	Number  DataType = "number"
	Integer DataType = "integer"
	String  DataType = "string"
	Array   DataType = "array"
	Null    DataType = "null"
	Boolean DataType = "boolean"
)

type Property struct {
	Type DataType `json:"type"`
	// Description is the description of the schema.
	Description string `json:"description,omitempty"`
	// Items specifies which data type an array contains, if the schema type is Array.
	Items *Property `json:"items,omitempty"`
	// Properties describes the properties of an object, if the schema type is Object.
	Properties map[string]*Property `json:"properties,omitempty"`
	Required   []string             `json:"required,omitempty"`
	Enum       []string             `json:"enum,omitempty"`
}

var schemaCache = pkg.SyncMap[*InputSchema]{}

func generateSchemaFromReqStruct(v any) (*InputSchema, error) {
	t := reflect.TypeOf(v)
	for t.Kind() != reflect.Struct {
		if t.Kind() != reflect.Ptr {
			return nil, fmt.Errorf("invalid type %v", t)
		}
		t = t.Elem()
	}

	typeUID := getTypeUUID(t)
	if schema, ok := schemaCache.Load(typeUID); ok {
		return schema, nil
	}

	schema := &InputSchema{Type: Object}

	property, err := reflectSchemaByObject(t)
	if err != nil {
		return nil, err
	}

	schema.Properties = property.Properties
	schema.Required = property.Required

	schemaCache.Store(typeUID, schema)
	return schema, nil
}

func getTypeUUID(t reflect.Type) string {
	if t.PkgPath() != "" && t.Name() != "" {
		return t.PkgPath() + "." + t.Name()
	}
	// fallback for unnamed types (like anonymous struct)
	return t.String()
}

func reflectSchemaByObject(t reflect.Type) (*Property, error) {
	var (
		properties      = make(map[string]*Property)
		requiredFields  = make([]string, 0)
		anonymousFields = make([]reflect.StructField, 0)
	)

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		if field.Anonymous {
			anonymousFields = append(anonymousFields, field)
			continue
		}

		if !field.IsExported() {
			continue
		}

		jsonTag := field.Tag.Get("json")
		if jsonTag == "-" {
			continue
		}
		required := true
		if jsonTag == "" {
			jsonTag = field.Name
		}
		if strings.HasSuffix(jsonTag, ",omitempty") {
			jsonTag = strings.TrimSuffix(jsonTag, ",omitempty")
			required = false
		}

		item, err := reflectSchemaByType(field.Type)
		if err != nil {
			return nil, err
		}

		if description := field.Tag.Get("description"); description != "" {
			item.Description = description
		}
		properties[jsonTag] = item

		if s := field.Tag.Get("required"); s != "" {
			required, err = strconv.ParseBool(s)
			if err != nil {
				return nil, fmt.Errorf("invalid required field %v: %v", jsonTag, err)
			}
		}
		if required {
			requiredFields = append(requiredFields, jsonTag)
		}

		if v := field.Tag.Get("enum"); v != "" {
			enumValues := strings.Split(v, ",")
			for j, value := range enumValues {
				enumValues[j] = strings.TrimSpace(value)
			}

			// Check if enum values are consistent with the field type
			for _, value := range enumValues {
				switch field.Type.Kind() {
				case reflect.String:
					// No additional processing required for string type
				case reflect.Int, reflect.Int64:
					if _, err := strconv.Atoi(value); err != nil {
						return nil, fmt.Errorf("enum value %q is not compatible with type %v", value, field.Type)
					}
				case reflect.Float64:
					if _, err := strconv.ParseFloat(value, 64); err != nil {
						return nil, fmt.Errorf("enum value %q is not compatible with type %v", value, field.Type)
					}
				default:
					return nil, fmt.Errorf("unsupported type %v for enum validation", field.Type)
				}
			}
			item.Enum = enumValues
		}
	}

	for _, field := range anonymousFields {
		object, err := reflectSchemaByObject(field.Type)
		if err != nil {
			return nil, err
		}
		for propName, propValue := range object.Properties {
			if _, ok := properties[propName]; ok {
				return nil, fmt.Errorf("duplicate property name %s in anonymous struct", propName)
			}
			properties[propName] = propValue
		}
		requiredFields = append(requiredFields, object.Required...)
	}

	property := &Property{
		Type:       ObjectT,
		Properties: properties,
		Required:   requiredFields,
	}
	return property, nil
}

func reflectSchemaByType(t reflect.Type) (*Property, error) {
	s := &Property{}

	switch t.Kind() {
	case reflect.String:
		s.Type = String
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		s.Type = Integer
	case reflect.Float32, reflect.Float64:
		s.Type = Number
	case reflect.Bool:
		s.Type = Boolean
	case reflect.Slice, reflect.Array:
		s.Type = Array
		items, err := reflectSchemaByType(t.Elem())
		if err != nil {
			return nil, err
		}
		s.Items = items
	case reflect.Struct:
		object, err := reflectSchemaByObject(t)
		if err != nil {
			return nil, err
		}
		object.Type = ObjectT
		s = object
	case reflect.Map:
		if t.Key().Kind() != reflect.String {
			return nil, fmt.Errorf("map key type %s is not supported", t.Key().Kind())
		}
		object := &Property{
			Type: ObjectT,
		}
		s = object
	case reflect.Ptr:
		p, err := reflectSchemaByType(t.Elem())
		if err != nil {
			return nil, err
		}
		s = p
	case reflect.Invalid, reflect.Uintptr, reflect.Complex64, reflect.Complex128,
		reflect.Chan, reflect.Func, reflect.Interface,
		reflect.UnsafePointer:
		return nil, fmt.Errorf("unsupported type: %s", t.Kind().String())
	default:
	}
	return s, nil
}
