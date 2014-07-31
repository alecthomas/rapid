package schema

import (
	"fmt"
	"net/url"
	"reflect"
	"strconv"
)

// EncodeStructToURLValues reflects over a struct, encoding it to a url.Values
// instance.
func EncodeStructToURLValues(i interface{}) (values url.Values) {
	values = url.Values{}
	if i == nil || reflect.ValueOf(i).IsNil() {
		return
	}
	if v, ok := i.(url.Values); ok {
		return v
	}
	v := reflect.Indirect(reflect.ValueOf(i))
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		tf := t.Field(i)
		name := tf.Tag.Get("schema")
		if name == "" {
			name = tf.Name
		}
		switch tf.Type.Kind() {
		case reflect.Slice:
			for j := 0; j < f.Len(); j++ {
				values.Add(name, encodeBaseType(f.Index(j)))
			}

		default:
			s := encodeBaseType(f)
			values.Set(name, s)
		}
	}
	return
}

func encodeBaseType(v reflect.Value) string {
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(v.Int(), 10)

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(v.Uint(), 10)

	case reflect.Float32:
		return strconv.FormatFloat(v.Float(), 'f', 4, 32)

	case reflect.Float64:
		return strconv.FormatFloat(v.Float(), 'f', 4, 64)

	case reflect.String:
		return v.String()
	}
	panic(fmt.Sprintf("unsupported type %v", v))
}
