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
