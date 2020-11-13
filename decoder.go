package bin

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"unicode/utf8"

	"go.uber.org/zap"
)

// UnmarshalerBinary is the interface implemented by types
// that can unmarshal an EOSIO binary description of themselves.
//
// **Warning** This is experimental, exposed only for internal usage for now.
type UnmarshalerBinary interface {
	UnmarshalBinary(decoder *Decoder) error
}

var TypeSize = struct {
	Bool int
	Byte int

	Int8  int
	Int16 int

	Uint8   int
	Uint16  int
	Uint32  int
	Uint64  int
	Uint128 int

	Float32 int
	Float64 int

	PublicKey int
	Signature int

	Tstamp         int
	BlockTimestamp int

	CurrencyName int
}{
	Byte: 1,
	Bool: 1,

	Int8:  1,
	Int16: 2,

	Uint8:   1,
	Uint16:  2,
	Uint32:  4,
	Uint64:  8,
	Uint128: 16,

	Float32: 4,
	Float64: 8,
}

// Decoder implements the EOS unpacking, similar to FC_BUFFER
type Decoder struct {
	data []byte
	pos  int
}

func NewDecoder(data []byte) *Decoder {
	return &Decoder{
		data: data,
	}
}

func (d *Decoder) Decode(v interface{}) (err error) {
	return d.decodeWithOption(v, nil)
}

func (d *Decoder) decodeWithOption(v interface{}, option *Option) (err error) {
	rv := reflect.ValueOf(v)
	//if rv.Kind() != reflect.Ptr || rv.IsNil() {
	//	return &InvalidDecoderError{reflect.TypeOf(v)}
	//}

	// We decode rv not rv.Elem because the Unmarshaler interface
	// test must be applied at the top level of the value.
	err = d.value(rv, option)
	if err != nil {
		return err
	}
	return nil
}
func (d *Decoder) value(rv reflect.Value, option *Option) (err error) {
	if option == nil {
		option = &Option{}
	}

	unmarshaler, rv := indirect(rv, true)
	rvType := rv.Type()

	if traceEnabled {
		zlog.Debug("decode type",
			zap.String("type", rvType.String()),
			zap.Reflect("options", option),
		)
	}

	if option.isOptional() {
		isPresent, e := d.ReadByte()
		if e != nil {
			err = fmt.Errorf("decode: %t isPresent, %s", rv.Type(), e)
			return
		}

		if isPresent == 0 {
			if traceEnabled {
				zlog.Debug("skipping optional value", typeField("type", rv))
			}

			rv.Set(reflect.Zero(rvType))
			return
		}
	}

	//if t.Kind() == reflect.Ptr {
	//	t = t.Elem()
	//	newRV := reflect.New(t)
	//	rv.Set(newRV)
	//
	//	// At this point, `newRV` is a pointer to our target type, we need to check here because
	//	// after that, when `reflect.Indirect` is used, we get a `**<Type>` instead of a `*<Type>`
	//	// which breaks the interface checking.
	//	//
	//	// Ultimately, I think this could should be re-written, I don't think the `**<Type>` is necessary.
	//	if u, ok := newRV.Interface().(UnmarshalerBinary); ok {
	//		if traceEnabled {
	//			zlog.Debug("using UnmarshalBinary method to decode type", typeField("type", v))
	//		}
	//		return u.UnmarshalBinary(d)
	//	}
	//
	//	rv = reflect.Indirect(newRV)
	//} else {
	//	// We check if `v` directly is `UnmarshalerBinary` this is to overcome our bad code that
	//	// has problem dealing with non-pointer type, which should still be possible here, by allocating
	//	// the empty value for it can then unmarshalling using the address of it. See comment above about
	//	// `newRV` being turned into `**<Type>`.
	//	//
	//	// We should re-code all the logic to determine the type and indirection using Golang `json` package
	//	// logic. See here: https://github.com/golang/go/blob/54697702e435bddb69c0b76b25b3209c78d2120a/src/encoding/json/decode.go#L439
	//	if u, ok := v.(UnmarshalerBinary); ok {
	//		if traceEnabled {
	//			zlog.Debug("using UnmarshalBinary method to decode type", typeField("type", v))
	//		}
	//		return u.UnmarshalBinary(d)
	//	}
	//}

	if unmarshaler != nil {
		if traceEnabled {
			zlog.Debug("using UnmarshalBinary method to decode type")
		}
		return unmarshaler.UnmarshalBinary(d)
	}

	switch rv.Kind() {
	case reflect.String:
		s, e := d.ReadString()
		if e != nil {
			err = e
			return
		}
		rv.SetString(s)
		return
	case reflect.Uint8:
		var n byte
		n, err = d.ReadByte()
		rv.SetUint(uint64(n))
		return
	case reflect.Int8:
		var n int8
		n, err = d.ReadInt8()
		rv.SetInt(int64(n))
		return
	case reflect.Int16:
		var n int16
		n, err = d.ReadInt16()
		rv.SetInt(int64(n))
		return
	case reflect.Int32:
		var n int32
		n, err = d.ReadInt32()
		rv.SetInt(int64(n))
		return
	case reflect.Int64:
		var n int64
		n, err = d.ReadInt64()
		rv.SetInt(int64(n))
		return
	//// This is so hackish, doing it right now, but the decoder needs to handle those
	//// case (a struct field that is itself a pointer) by itself.
	//case **Uint64:
	//	var n uint64
	//	n, err = d.ReadUint64()
	//	if err == nil {
	//		rv.Set(reflect.ValueOf((Uint64)(n)))
	//	}
	//	return
	case reflect.Uint16:
		var n uint16
		n, err = d.ReadUint16()
		rv.SetUint(uint64(n))
		return
	case reflect.Uint32:
		var n uint32
		n, err = d.ReadUint32()
		rv.SetUint(uint64(n))
		return
	case reflect.Uint64:
		var n uint64
		n, err = d.ReadUint64()
		rv.SetUint(n)
		return
	//case *Varint16:
	//	var r int16
	//	r, err = d.ReadVarint16()
	//	rv.SetInt(int64(r))
	//	return
	//case *Varuint16:
	//	var r uint16
	//	r, err = d.ReadUvarint16()
	//	rv.SetUint(uint64(r))
	//	return
	case reflect.Float32:
		var n float32
		n, err = d.ReadFloat32()
		rv.SetFloat(float64(n))
		return
	case reflect.Float64:
		var n float64
		n, err = d.ReadFloat64()
		rv.SetFloat(n)
		return
	case reflect.Bool:
		var r bool
		r, err = d.ReadBool()
		rv.SetBool(r)
		return
		//case *[]byte:
		//	var data []byte
		//	data, err = d.ReadByteArray()
		//	rv.SetBytes(data)
		//	return
	}

	switch rvType.Kind() {
	case reflect.Array:
		len := rvType.Len()

		if traceEnabled {
			zlog.Debug("reading array", zap.Int("length", len))
		}
		for i := 0; i < len; i++ {
			if err = d.decodeWithOption(rv.Index(i).Addr().Interface(), nil); err != nil {
				return
			}
		}
		return
	case reflect.Slice:
		var l int
		if option.hasSizeOfSlice() {
			l = option.getSizeOfSlice()
		} else {
			length, err := d.ReadUvarint64()
			if err != nil {
				return err
			}
			l = int(length)
		}
		if traceEnabled {
			zlog.Debug("reading slice", zap.Int("len", l), typeField("type", rv))
		}
		rv.Set(reflect.MakeSlice(rvType, int(l), int(l)))
		for i := 0; i < int(l); i++ {
			if err = d.decodeWithOption(rv.Index(i).Addr().Interface(), nil); err != nil {
				return
			}
		}

	case reflect.Struct:

		err = d.decodeStruct(rvType, rv)
		if err != nil {
			return
		}

	default:
		return fmt.Errorf("decode: unsupported type %q", rvType)
	}

	return
}

// rv is the instance of the structure
// t is the type of the structure
func (d *Decoder) decodeStruct(rt reflect.Type, rv reflect.Value) (err error) {
	l := rv.NumField()

	sizeOfMap := map[string]int{}
	seenBinaryExtensionField := false
	for i := 0; i < l; i++ {
		structField := rt.Field(i)

		fieldTag := parseFieldTag(structField.Tag)
		if fieldTag.Skip {
			continue
		}

		if !fieldTag.BinaryExtension && seenBinaryExtensionField {
			panic(fmt.Sprintf("the `bin:\"binary_extension\"` tags must be packed together at the end of struct fields, problematic field %q", structField.Name))
		}

		if fieldTag.BinaryExtension {
			seenBinaryExtensionField = true
			// FIXME: This works only if what is in `d.data` is the actual full data buffer that
			//        needs to be decoded. If there is for example two structs in the buffer, this
			//        will not work as we would continue into the next struct.
			//
			//        But at the same time, does it make sense otherwise? What would be the inference
			//        rule in the case of extra bytes available? Continue decoding and revert if it's
			//        not working? But how to detect valid errors?
			if len(d.data[d.pos:]) <= 0 {
				continue
			}
		}

		if v := rv.Field(i); v.CanSet() && structField.Name != "_" {
			option := &Option{}

			if s, ok := sizeOfMap[structField.Name]; ok {
				option.setSizeOfSlice(s)
			}

			// v is Value of given field for said struct
			if fieldTag.Optional {
				option.OptionalField = true
			}

			// creates a pointer to the value.....
			value := v.Addr().Interface()

			if traceEnabled {
				zlog.Debug("struct field",
					typeField(structField.Name, value),
					zap.Reflect("field_tags", fieldTag),
				)
			}

			if err = d.decodeWithOption(value, option); err != nil {
				return
			}

			if fieldTag.Sizeof != "" {
				size := sizeof(structField.Type, v)
				if traceEnabled {
					zlog.Debug("setting size of field",
						zap.String("field_name", fieldTag.Sizeof),
						zap.Int("size", size),
					)
				}
				sizeOfMap[fieldTag.Sizeof] = size
			}
		}
	}
	return
}

func sizeof(t reflect.Type, v reflect.Value) int {
	switch t.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return int(v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n := int(v.Uint())
		// all the builtin array length types are native int
		// so this guards against weird truncation
		if n < 0 {
			return 0
		}
		return n
	default:
		//name := v.Type().FieldByIndex(index).Name
		//panic(fmt.Sprintf("sizeof field %T.%s not an integer type", val.Interface(), name))
		panic(fmt.Sprintf("sizeof field "))
	}
}

var ErrVarIntBufferSize = errors.New("varint: invalid buffer size")

func (d *Decoder) ReadUvarint64() (uint64, error) {
	l, read := binary.Uvarint(d.data[d.pos:])
	if read <= 0 {
		return l, ErrVarIntBufferSize
	}
	if traceEnabled {
		zlog.Debug("read uvarint64", zap.Uint64("val", l))
	}
	d.pos += read
	return l, nil
}

func (d *Decoder) ReadVarint64() (out int64, err error) {
	l, read := binary.Varint(d.data[d.pos:])
	if read <= 0 {
		return l, ErrVarIntBufferSize
	}
	if traceEnabled {
		zlog.Debug("read varint", zap.Int64("val", l))
	}
	d.pos += read
	return l, nil
}

func (d *Decoder) ReadVarint32() (out int32, err error) {
	n, err := d.ReadVarint64()
	if err != nil {
		return out, err
	}
	out = int32(n)
	if traceEnabled {
		zlog.Debug("read varint32", zap.Int32("val", out))
	}
	return
}

func (d *Decoder) ReadUvarint32() (out uint32, err error) {

	n, err := d.ReadUvarint64()
	if err != nil {
		return out, err
	}
	out = uint32(n)
	if traceEnabled {
		zlog.Debug("read uvarint32", zap.Uint32("val", out))
	}
	return
}
func (d *Decoder) ReadVarint16() (out int16, err error) {
	n, err := d.ReadVarint64()
	if err != nil {
		return out, err
	}
	out = int16(n)
	if traceEnabled {
		zlog.Debug("read varint16", zap.Int16("val", out))
	}
	return
}

func (d *Decoder) ReadUvarint16() (out uint16, err error) {

	n, err := d.ReadUvarint64()
	if err != nil {
		return out, err
	}
	out = uint16(n)
	if traceEnabled {
		zlog.Debug("read uvarint16", zap.Uint16("val", out))
	}
	return
}

func (d *Decoder) ReadByteArray() (out []byte, err error) {

	l, err := d.ReadUvarint64()
	if err != nil {
		return nil, err
	}

	if len(d.data) < d.pos+int(l) {
		return nil, fmt.Errorf("byte array: varlen=%d, missing %d bytes", l, d.pos+int(l)-len(d.data))
	}

	out = d.data[d.pos : d.pos+int(l)]
	d.pos += int(l)
	if traceEnabled {
		zlog.Debug("read byte array", zap.Stringer("hex", HexBytes(out)))
	}
	return
}

func (d *Decoder) ReadByte() (out byte, err error) {
	if d.remaining() < TypeSize.Byte {
		err = fmt.Errorf("required [1] byte, remaining [%d]", d.remaining())
		return
	}

	out = d.data[d.pos]
	d.pos++
	if traceEnabled {
		zlog.Debug("read byte", zap.Uint8("byte", out), zap.String("hex", hex.EncodeToString([]byte{out})))
	}
	return
}

func (d *Decoder) ReadBool() (out bool, err error) {
	if d.remaining() < TypeSize.Bool {
		err = fmt.Errorf("bool required [%d] byte, remaining [%d]", TypeSize.Bool, d.remaining())
		return
	}

	b, err := d.ReadByte()

	if err != nil {
		err = fmt.Errorf("readBool, %s", err)
	}
	out = b != 0
	if traceEnabled {
		zlog.Debug("read bool", zap.Bool("val", out))
	}
	return

}

func (d *Decoder) ReadUint8() (out uint8, err error) {
	out, err = d.ReadByte()
	return
}

func (d *Decoder) ReadInt8() (out int8, err error) {
	b, err := d.ReadByte()
	out = int8(b)
	if traceEnabled {
		zlog.Debug("read int8", zap.Int8("val", out))
	}
	return
}

func (d *Decoder) ReadUint16() (out uint16, err error) {
	if d.remaining() < TypeSize.Uint16 {
		err = fmt.Errorf("uint16 required [%d] bytes, remaining [%d]", TypeSize.Uint16, d.remaining())
		return
	}

	out = binary.LittleEndian.Uint16(d.data[d.pos:])
	d.pos += TypeSize.Uint16
	if traceEnabled {
		zlog.Debug("read uint16", zap.Uint16("val", out))
	}
	return
}

func (d *Decoder) ReadInt16() (out int16, err error) {
	n, err := d.ReadUint16()
	out = int16(n)
	if traceEnabled {
		zlog.Debug("read int16", zap.Int16("val", out))
	}
	return
}

func (d *Decoder) ReadInt64() (out int64, err error) {
	n, err := d.ReadUint64()
	out = int64(n)
	if traceEnabled {
		zlog.Debug("read int64", zap.Int64("val", out))
	}
	return
}

func (d *Decoder) ReadUint32() (out uint32, err error) {
	if d.remaining() < TypeSize.Uint32 {
		err = fmt.Errorf("uint32 required [%d] bytes, remaining [%d]", TypeSize.Uint32, d.remaining())
		return
	}

	out = binary.LittleEndian.Uint32(d.data[d.pos:])
	d.pos += TypeSize.Uint32
	if traceEnabled {
		zlog.Debug("read uint32", zap.Uint32("val", out))
	}
	return
}

func (d *Decoder) ReadInt32() (out int32, err error) {
	n, err := d.ReadUint32()
	out = int32(n)
	if traceEnabled {
		zlog.Debug("read int32", zap.Int32("val", out))
	}
	return
}

func (d *Decoder) ReadUint64() (out uint64, err error) {
	if d.remaining() < TypeSize.Uint64 {
		err = fmt.Errorf("uint64 required [%d] bytes, remaining [%d]", TypeSize.Uint64, d.remaining())
		return
	}

	data := d.data[d.pos : d.pos+TypeSize.Uint64]
	out = binary.LittleEndian.Uint64(data)
	d.pos += TypeSize.Uint64
	if traceEnabled {
		zlog.Debug("read uint64", zap.Uint64("val", out), zap.Stringer("hex", HexBytes(data)))
	}
	return
}

func (d *Decoder) ReadInt128() (out Int128, err error) {
	v, err := d.ReadUint128("int128")
	if err != nil {
		return
	}

	return Int128(v), nil
}

func (d *Decoder) ReadUint128(typeName string) (out Uint128, err error) {
	if d.remaining() < TypeSize.Uint128 {
		err = fmt.Errorf("%s required [%d] bytes, remaining [%d]", typeName, TypeSize.Uint128, d.remaining())
		return
	}

	data := d.data[d.pos : d.pos+TypeSize.Uint128]
	out.Lo = binary.LittleEndian.Uint64(data)
	out.Hi = binary.LittleEndian.Uint64(data[8:])

	d.pos += TypeSize.Uint128
	if traceEnabled {
		zlog.Debug("read uint128", zap.Stringer("hex", out), zap.Uint64("hi", out.Hi), zap.Uint64("lo", out.Lo))
	}
	return
}

func (d *Decoder) ReadFloat32() (out float32, err error) {
	if d.remaining() < TypeSize.Float32 {
		err = fmt.Errorf("float32 required [%d] bytes, remaining [%d]", TypeSize.Float32, d.remaining())
		return
	}

	n := binary.LittleEndian.Uint32(d.data[d.pos:])
	out = math.Float32frombits(n)
	d.pos += TypeSize.Float32
	if traceEnabled {
		zlog.Debug("read float32", zap.Float32("val", out))
	}
	return
}

func (d *Decoder) ReadFloat64() (out float64, err error) {
	if d.remaining() < TypeSize.Float64 {
		err = fmt.Errorf("float64 required [%d] bytes, remaining [%d]", TypeSize.Float64, d.remaining())
		return
	}

	n := binary.LittleEndian.Uint64(d.data[d.pos:])
	out = math.Float64frombits(n)
	d.pos += TypeSize.Float64
	if traceEnabled {
		zlog.Debug("read Float64", zap.Float64("val", float64(out)))
	}
	return
}

func (d *Decoder) ReadFloat128() (out Float128, err error) {
	value, err := d.ReadUint128("float128")
	if err != nil {
		return out, fmt.Errorf("float128: %s", err)
	}

	return Float128(value), nil
}

func (d *Decoder) SafeReadUTF8String() (out string, err error) {
	data, err := d.ReadByteArray()
	out = strings.Map(fixUtf, string(data))
	if traceEnabled {
		zlog.Debug("read safe UTF8 string", zap.String("val", out))
	}
	return
}

func (d *Decoder) ReadString() (out string, err error) {
	data, err := d.ReadByteArray()
	out = string(data)
	if traceEnabled {
		zlog.Debug("read string", zap.String("val", out))
	}
	return
}

func (d *Decoder) remaining() int {
	return len(d.data) - d.pos
}

func (d *Decoder) hasRemaining() bool {
	return d.remaining() > 0
}

//func UnmarshalBinaryReader(reader io.Reader, v interface{}) (err error) {
//	data, err := ioutil.ReadAll(reader)
//	if err != nil {
//		return
//	}
//	return UnmarshalBinary(data, v)
//}
//
//func UnmarshalBinary(data []byte, v interface{}) (err error) {
//	decoder := NewDecoder(data)
//	return decoder.Decode(v)
//}

func fixUtf(r rune) rune {
	if r == utf8.RuneError {
		return '�'
	}
	return r
}

// indirect walks down v allocating pointers as needed,
// until it gets to a non-pointer.
// if it encounters an Unmarshaler, indirect stops and returns that.
// if decodingNull is true, indirect stops at the last pointer so it can be set to nil.
//
// *Note* This is a copy of `encoding/json/decoder.go#indirect` of Golang 1.14.
//
// See here: https://github.com/golang/go/blob/go1.14.2/src/encoding/json/decode.go#L439
func indirect(v reflect.Value, decodingNull bool) (UnmarshalerBinary, reflect.Value) {
	// Issue #24153 indicates that it is generally not a guaranteed property
	// that you may round-trip a reflect.Value by calling Value.Addr().Elem()
	// and expect the value to still be settable for values derived from
	// unexported embedded struct fields.
	//
	// The logic below effectively does this when it first addresses the value
	// (to satisfy possible pointer methods) and continues to dereference
	// subsequent pointers as necessary.
	//
	// After the first round-trip, we set v back to the original value to
	// preserve the original RW flags contained in reflect.Value.
	v0 := v
	haveAddr := false

	// If v is a named type and is addressable,
	// start with its address, so that if the type has pointer methods,
	// we find them.
	if v.Kind() != reflect.Ptr && v.Type().Name() != "" && v.CanAddr() {
		haveAddr = true
		v = v.Addr()
	}
	for {
		// Load value from interface, but only if the result will be
		// usefully addressable.
		if v.Kind() == reflect.Interface && !v.IsNil() {
			e := v.Elem()
			if e.Kind() == reflect.Ptr && !e.IsNil() && (!decodingNull || e.Elem().Kind() == reflect.Ptr) {
				haveAddr = false
				v = e
				continue
			}
		}

		if v.Kind() != reflect.Ptr {
			break
		}

		if v.Elem().Kind() != reflect.Ptr && decodingNull && v.CanSet() {
			break
		}

		// Prevent infinite loop if v is an interface pointing to its own address:
		//     var v interface{}
		//     v = &v
		if v.Elem().Kind() == reflect.Interface && v.Elem().Elem() == v {
			v = v.Elem()
			break
		}
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		if v.Type().NumMethod() > 0 && v.CanInterface() {
			if u, ok := v.Interface().(UnmarshalerBinary); ok {
				return u, reflect.Value{}
			}
		}

		if haveAddr {
			v = v0 // restore original value after round-trip Value.Addr().Elem()
			haveAddr = false
		} else {
			v = v.Elem()
		}
	}
	return nil, v
}
