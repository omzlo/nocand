package properties

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
)

//
// Properties
//

const (
	PROP_TYPE_NULL byte = iota
	PROP_TYPE_BOOL
	PROP_TYPE_STRING
	PROP_TYPE_INT32
	PROP_TYPE_UINT32
	PROP_TYPE_FLOAT32
)

type Property struct {
	Type  byte        `json:"-"`
	Value interface{} `json:"value"`
}

type Properties struct {
	Properties map[string]Property
}

func New() *Properties {
	return &Properties{Properties: make(map[string]Property)}
}

func (si *Properties) Clear() {
	// optimized by compiler since Go 1.11
	for k := range si.Properties {
		delete(si.Properties, k)
	}
}

func (si *Properties) Add(k string, t byte, v interface{}) bool {
	if len(k) > 127 {
		return false
	}
	si.Properties[k] = Property{t, v}
	return true
}

func (si *Properties) AddNil(k string) bool {
	return si.Add(k, PROP_TYPE_NULL, nil)
}

func (si *Properties) AddString(k string, v string) bool {
	return si.Add(k, PROP_TYPE_STRING, v)
}

func (si *Properties) AddInt32(k string, v int32) bool {
	return si.Add(k, PROP_TYPE_INT32, v)
}

func (si *Properties) AddUint32(k string, v uint32) bool {
	return si.Add(k, PROP_TYPE_UINT32, v)
}

func (si *Properties) AddFloat32(k string, v float32) bool {
	return si.Add(k, PROP_TYPE_FLOAT32, v)
}

func (si *Properties) AddBool(k string, v bool) bool {
	return si.Add(k, PROP_TYPE_BOOL, v)
}

func (si *Properties) MarshalJSON() ([]byte, error) {
	return json.Marshal(si.Properties)
}

func (si *Properties) AsString(k string) string {
	if x, ok := si.Properties[k]; ok {
		return fmt.Sprintf("%v", x.Value)
	}
	return ""
}

func (si *Properties) PackValue() ([]byte, error) {
	buf := new(bytes.Buffer)

	for key, prop := range si.Properties {
		buf.WriteByte(byte(len(key)))
		buf.WriteString(key)

		buf.WriteByte(prop.Type)
		switch prop.Type {
		case PROP_TYPE_NULL:
			// do noithing
		case PROP_TYPE_INT32:
			binary.Write(buf, binary.BigEndian, prop.Value.(int32))
		case PROP_TYPE_UINT32:
			binary.Write(buf, binary.BigEndian, prop.Value.(uint32))
		case PROP_TYPE_FLOAT32:
			binary.Write(buf, binary.BigEndian, prop.Value.(uint32))
		case PROP_TYPE_BOOL:
			binary.Write(buf, binary.BigEndian, prop.Value.(bool))
		case PROP_TYPE_STRING:
			s := prop.Value.(string)
			buf.WriteByte(byte(len(s)))
			buf.WriteString(s)
		default:
			return nil, fmt.Errorf("Unexpected type %d while packing Property", prop.Type)
		}
	}
	return buf.Bytes(), nil
}

func (si *Properties) UnpackValue(b []byte) error {
	var len byte
	var ptype byte
	var err error

	si.Clear()

	buf := bytes.NewReader(b)

	for buf.Len() > 0 {
		if len, err = buf.ReadByte(); err != nil {
			return err
		}
		key := make([]byte, len)
		if _, err := buf.Read(key); err != nil {
			return err
		}
		if ptype, err = buf.ReadByte(); err != nil {
			return err
		}
		switch ptype {
		case PROP_TYPE_INT32:
			var i int32
			binary.Read(buf, binary.BigEndian, &i)
			si.AddInt32(string(key), i)
		case PROP_TYPE_UINT32:
			var u uint32
			binary.Read(buf, binary.BigEndian, &u)
			si.AddUint32(string(key), u)
		case PROP_TYPE_FLOAT32:
			var f float32
			binary.Read(buf, binary.BigEndian, &f)
			si.AddFloat32(string(key), f)
		case PROP_TYPE_BOOL:
			var b bool
			binary.Read(buf, binary.BigEndian, &b)
			si.AddBool(string(key), b)
		case PROP_TYPE_STRING:
			var slen byte
			if len, err = buf.ReadByte(); err != nil {
				return err
			}
			sval := make([]byte, slen)
			if _, err := buf.Read(sval); err != nil {
				return err
			}
			si.AddString(string(key), string(sval))
		default:
			return fmt.Errorf("Unexpected type %d unpacking Property", ptype)
		}
	}
	return nil
}
