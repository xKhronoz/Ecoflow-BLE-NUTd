package ecoflow

import (
	"google.golang.org/protobuf/reflect/protoreflect"
)

func protoFloat32Field(msg protoreflect.ProtoMessage, names ...string) (float32, bool) {
	m := msg.ProtoReflect()
	fields := m.Descriptor().Fields()
	for _, name := range names {
		fd := fields.ByName(protoreflect.Name(name))
		if fd == nil {
			continue
		}
		if fd.HasPresence() && !m.Has(fd) {
			continue
		}
		return float32(m.Get(fd).Float()), true
	}
	return 0, false
}

func protoUint32Field(msg protoreflect.ProtoMessage, names ...string) (uint32, bool) {
	m := msg.ProtoReflect()
	fields := m.Descriptor().Fields()
	for _, name := range names {
		fd := fields.ByName(protoreflect.Name(name))
		if fd == nil {
			continue
		}
		if fd.HasPresence() && !m.Has(fd) {
			continue
		}
		return uint32(m.Get(fd).Uint()), true
	}
	return 0, false
}

func protoBoolField(msg protoreflect.ProtoMessage, names ...string) (bool, bool) {
	m := msg.ProtoReflect()
	fields := m.Descriptor().Fields()
	for _, name := range names {
		fd := fields.ByName(protoreflect.Name(name))
		if fd == nil {
			continue
		}
		if fd.HasPresence() && !m.Has(fd) {
			continue
		}
		return m.Get(fd).Bool(), true
	}
	return false, false
}

func protoInt32Field(msg protoreflect.ProtoMessage, names ...string) (int32, bool) {
	m := msg.ProtoReflect()
	fields := m.Descriptor().Fields()
	for _, name := range names {
		fd := fields.ByName(protoreflect.Name(name))
		if fd == nil {
			continue
		}
		if fd.HasPresence() && !m.Has(fd) {
			continue
		}
		return int32(m.Get(fd).Int()), true
	}
	return 0, false
}

// protoNumberField reads the first present field of names whatever its
// numeric kind: the property messages mix float, int32 and uint32 for
// alike readings (a temperature is int32 in one message and float in another).
func protoNumberField(msg protoreflect.ProtoMessage, names ...string) (float64, bool) {
	m := msg.ProtoReflect()
	fields := m.Descriptor().Fields()
	for _, name := range names {
		fd := fields.ByName(protoreflect.Name(name))
		if fd == nil {
			continue
		}
		if fd.HasPresence() && !m.Has(fd) {
			continue
		}
		value := m.Get(fd)
		switch fd.Kind() {
		case protoreflect.FloatKind, protoreflect.DoubleKind:
			return value.Float(), true
		case protoreflect.Int32Kind, protoreflect.Int64Kind, protoreflect.Sint32Kind, protoreflect.Sint64Kind,
			protoreflect.Sfixed32Kind, protoreflect.Sfixed64Kind:
			return float64(value.Int()), true
		case protoreflect.Uint32Kind, protoreflect.Uint64Kind, protoreflect.Fixed32Kind, protoreflect.Fixed64Kind:
			return float64(value.Uint()), true
		}
	}
	return 0, false
}

// protoNumberListField reads a repeated numeric field inside a message-typed
// field (the per-group lists: pow_get_ac_out_list holds one
// pow_get_ac_out_item per AC outlet group).
func protoNumberListField(msg protoreflect.ProtoMessage, field string, item string) ([]float64, bool) {
	m := msg.ProtoReflect()
	fd := m.Descriptor().Fields().ByName(protoreflect.Name(field))
	if fd == nil || fd.Kind() != protoreflect.MessageKind || (fd.HasPresence() && !m.Has(fd)) {
		return nil, false
	}
	inner := m.Get(fd).Message()
	ifd := inner.Descriptor().Fields().ByName(protoreflect.Name(item))
	if ifd == nil || !ifd.IsList() {
		return nil, false
	}
	list := inner.Get(ifd).List()
	out := make([]float64, list.Len())
	for i := range out {
		v := list.Get(i)
		switch ifd.Kind() {
		case protoreflect.FloatKind, protoreflect.DoubleKind:
			out[i] = v.Float()
		case protoreflect.Int32Kind, protoreflect.Int64Kind, protoreflect.Sint32Kind, protoreflect.Sint64Kind,
			protoreflect.Sfixed32Kind, protoreflect.Sfixed64Kind:
			out[i] = float64(v.Int())
		case protoreflect.Uint32Kind, protoreflect.Uint64Kind, protoreflect.Fixed32Kind, protoreflect.Fixed64Kind:
			out[i] = float64(v.Uint())
		default:
			return nil, false
		}
	}
	return out, true
}
