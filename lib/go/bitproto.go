// Copyright (c) 2021, hit9. https://github.com/hit9/bitproto

// Package bitproto is the encoding library for bitproto in Go language.
//
// Keep it simple:
//   - Pure golang.
//   - No reflection.
//   - No type assertion.
//   - No dynamic function construction.
package bitproto

// Exported for generated go files to reference to avoid bp imported but not used error.
var Useless = false

// Flag
type Flag = int

const (
	FlagBool         Flag = 1
	FlagInt               = 2
	FlagUint              = 3
	FlagByte              = 4
	FlagEnum              = 5
	FlagAlias             = 6
	FlagArray             = 7
	FlagMessage           = 8
	FlagMessageField      = 9
)

// ProcessContext is the context accross all processor functions in a encoding
// or decoding process.
type ProcessContext struct {
	// Indicates whether current processing is encoding.
	isEncode bool
	// Tracks the number of total bits processed.
	i int
	// Bytes buffer processing.
	// When encoding, s is the destination buffer to write.
	// When decoding, s is the source buffer to read.
	s []byte
}

// NewEncodeContext returns a ProcessContext for encoding to given buffer s.
func NewEncodeContext(nbytes int) *ProcessContext {
	return &ProcessContext{true, 0, make([]byte, nbytes)}
}

// NewDecodeContext returns a ProcessContext for decoding from given buffer s.
func NewDecodeContext(s []byte) *ProcessContext {
	return &ProcessContext{false, 0, s}
}

func (ctx *ProcessContext) Buffer() []byte { return ctx.s }

// Processor is the abstraction type that able to process encoding and
// decoding.
type Processor interface {
	// Flag returns the processor flag.
	Flag() Flag
	// Process continues the encoding and decoding with given context.
	// The argument di and accessor is to index data to read and write byte.
	Process(ctx *ProcessContext, di *DataIndexer, accessor Accessor)
}

// Accessor is the data container.
// Assuming the compiler generates these functions for messages.
// We don't use reflection (the "encoding/json" way, which slows the
// performance) since there's already a bitproto compiler to generate code.
type Accessor interface {
	// BpSetByte sets given byte b to target data, the data will be lookedup
	// by given indexer di from this accessor.
	// Argument lshift is the number of bits to shift right on b before it's
	// written onto the indexed data.
	// This function works only if target data is a single type.
	BpSetByte(di *DataIndexer, lshift int, b byte)

	// BpGetByte returns the byte from the data lookedup by given indexer di
	// from this accessor.
	// Argument rshift is the number of bits to shift left on the data before
	// it's cast to byte.
	// This function works only if target data is a single type.
	BpGetByte(di *DataIndexer, rshift int) byte

	// BpGetAccessor gets the child accessor data container by indexer di.
	// This function works only if target data is a message.
	BpGetAccessor(di *DataIndexer) Accessor

	// BpProcessInt processes the signed integers right after bite coping is done.
	BpProcessInt(di *DataIndexer)
}

// Uint8Accessor implements Accessor for uint8 value encoding and decoding.
type Uint8Accessor struct{ data uint8 }

func (m *Uint8Accessor) BpSetByte(di *DataIndexer, lshift int, b byte) {
	switch di.F() {
	case 1:
		m.data |= (uint8(b) << lshift)
	}
}
func (m *Uint8Accessor) BpGetByte(di *DataIndexer, rshift int) byte {
	switch di.F() {
	case 1:
		return byte(m.data >> rshift)
	default:
		return byte(0)
	}
}
func (m *Uint8Accessor) BpGetAccessor(di *DataIndexer) Accessor { return nil }
func (m *Uint8Accessor) BpProcessInt(di *DataIndexer)           { return }

// Uint16Accessor implements Accessor for uint16 value encoding and decoding.
type Uint16Accessor struct{ data uint16 }

func (m *Uint16Accessor) BpSetByte(di *DataIndexer, lshift int, b byte) {
	switch di.F() {
	case 1:
		m.data |= (uint16(b) << lshift)
	}
}
func (m *Uint16Accessor) BpGetByte(di *DataIndexer, rshift int) byte {
	switch di.F() {
	case 1:
		return byte(m.data >> rshift)
	default:
		return byte(0)
	}
}
func (m *Uint16Accessor) BpGetAccessor(di *DataIndexer) Accessor { return nil }
func (m *Uint16Accessor) BpProcessInt(di *DataIndexer)           { return }

// DataIndexer contains the argument to index data from current accessor.
type DataIndexer struct {
	fnumber int
	aistack []int // array index stack in case of nested array in a single message.
}

// NewDataIndexer returns a new DataIndexer.
func NewDataIndexer(fnumber int) *DataIndexer { return &DataIndexer{fnumber, []int{}} }
func (di *DataIndexer) F() int                { return di.fnumber }
func (di *DataIndexer) I(n int) int           { return di.aistack[n] }
func (di *DataIndexer) IndexStackUp()         { di.aistack = append(di.aistack, 0) }
func (di *DataIndexer) IndexStackDown()       { di.aistack = di.aistack[0 : len(di.aistack)-1] }
func (di *DataIndexer) IndexReplace(k int)    { di.aistack[len(di.aistack)-1] = k }

// Bool implements Processor for bool type.
type Bool struct{}

func NewBool() *Bool       { return &Bool{} }
func (t *Bool) Flag() Flag { return FlagBool }
func (t *Bool) Process(ctx *ProcessContext, di *DataIndexer, accessor Accessor) {
	processBaseType(1, ctx, di, accessor)
}

// Int implements Processor for int type.
type Int struct{ nbits int }

func NewInt(nbits int) *Int { return &Int{nbits} }
func (t *Int) Flag() Flag   { return FlagInt }
func (t *Int) Process(ctx *ProcessContext, di *DataIndexer, accessor Accessor) {
	// Copy bits
	processBaseType(t.nbits, ctx, di, accessor)

	// Process the sign bit
	if ctx.isEncode {
		return
	}
	accessor.BpProcessInt(di)
}

// Uint implements Processor for uint type.
type Uint struct{ nbits int }

func NewUint(nbits int) *Uint { return &Uint{nbits} }
func (t *Uint) Flag() Flag    { return FlagUint }
func (t *Uint) Process(ctx *ProcessContext, di *DataIndexer, accessor Accessor) {
	processBaseType(t.nbits, ctx, di, accessor)
}

// Byte implements Processor for byte type.
type Byte struct{}

func NewByte() *Byte       { return &Byte{} }
func (t *Byte) Flag() Flag { return FlagByte }
func (t *Byte) Process(ctx *ProcessContext, di *DataIndexer, accessor Accessor) {
	processBaseType(8, ctx, di, accessor)
}

// Array implements Processor for array type.
type Array struct {
	extensible       bool
	capacity         int
	elementProcessor Processor
}

func NewArray(extensible bool, capacity int, elementProcessor Processor) *Array {
	return &Array{
		extensible, capacity, elementProcessor,
	}
}
func (t *Array) Flag() Flag { return FlagArray }

func (t *Array) Process(ctx *ProcessContext, di *DataIndexer, accessor Accessor) {
	di.IndexStackUp()
	defer di.IndexStackDown()

	// Record current number of bits processed.
	i := ctx.i
	// Opponent array capacity if extensible set.
	ahead := uint16(0)

	if t.extensible {
		if ctx.isEncode {
			// Encode ahead flag if extensible.
			t.EncodeExtensibleAhead(ctx)
		} else {
			// Decode ahead flag if extensible.
			ahead = t.DecodeExtensibleAhead(ctx)
		}
	}

	// Process array elements.
	for k := 0; k < t.capacity; k++ {
		// Rewrite indexer's array index tracker.
		di.IndexReplace(k)
		t.elementProcessor.Process(ctx, di, accessor)
	}

	// Skip redundant bits post decoding.
	if t.extensible && !ctx.isEncode {
		// Skip redundant bits.
		ito := i + int(ahead)*t.capacity
		if ito >= ctx.i {
			ctx.i = ito
		}
	}
}

// EncodeExtensibleAhead encode the array capacity as the ahead flag to current
// bit encoding stream.
func (t *Array) EncodeExtensibleAhead(ctx *ProcessContext) {
	// Safe to cast:
	// the capacity of an array always <= 65535.
	data := uint16(t.capacity)
	accessor := &Uint16Accessor{data}
	di := NewDataIndexer(1)
	processBaseType(16, ctx, di, accessor)
}

// DecodeExtensibleAhead decode the ahead flag as the array capacity from
// current decoding stream.
func (t *Array) DecodeExtensibleAhead(ctx *ProcessContext) uint16 {
	accessor := &Uint16Accessor{}
	di := NewDataIndexer(1)
	processBaseType(16, ctx, di, accessor)
	return accessor.data
}

// EnumProcessor implements Processor for enum.
// Assuming compiler generates Enum a method: BpProcessor to returns this.
type EnumProcessor struct {
	ut *Uint
}

func NewEnumProcessor(ut *Uint) *EnumProcessor {
	return &EnumProcessor{ut}
}

func (t *EnumProcessor) Flag() Flag { return FlagEnum }

func (t *EnumProcessor) Process(ctx *ProcessContext, di *DataIndexer, accessor Accessor) {
	// Process inner uint.
	t.ut.Process(ctx, di, accessor)
}

// AliasProcessor implements Processor for alias.
// Assuming compiler generates Alias a method: BpProcessor to returns this.
type AliasProcessor struct{ to Processor }

func NewAliasProcessor(to Processor) *AliasProcessor { return &AliasProcessor{to} }

func (t *AliasProcessor) Flag() Flag { return FlagAlias }

func (t *AliasProcessor) Process(ctx *ProcessContext, di *DataIndexer, accessor Accessor) {
	t.to.Process(ctx, di, accessor)
}

// MessageFieldProcessor implements Processor for message field.
type MessageFieldProcessor struct {
	fieldNumber   int
	typeProcessor Processor
}

func NewMessageFieldProcessor(fieldNumber int, typeProcessor Processor) *MessageFieldProcessor {
	return &MessageFieldProcessor{fieldNumber, typeProcessor}
}

func (t *MessageFieldProcessor) Flag() Flag { return FlagMessageField }

func (t *MessageFieldProcessor) Process(ctx *ProcessContext, _ *DataIndexer, accessor Accessor) {
	// Ignore data indexer passed in, because accessor is rewrite.
	// Rewrite data indexer's fieldNumber.
	di := NewDataIndexer(t.fieldNumber)
	t.typeProcessor.Process(ctx, di, accessor)
}

// MessageProcessor implements Processor for message
// Assuming compiler generates Message a method: BpProcessor to returns this.
type MessageProcessor struct {
	extensible       bool
	nbits            int
	fieldDescriptors []*MessageFieldProcessor
}

func NewMessageProcessor(extensible bool, nbits int, fieldDescriptors []*MessageFieldProcessor) *MessageProcessor {
	return &MessageProcessor{extensible, nbits, fieldDescriptors}
}

func (t *MessageProcessor) Flag() Flag { return FlagMessage }

func (t *MessageProcessor) Process(ctx *ProcessContext, di *DataIndexer, accessor Accessor) {
	if di != nil {
		// As a data item of upper accessor.
		// Rewrite accessor.
		accessor = accessor.BpGetAccessor(di)
	}

	// Record current number of bits processed.
	i := ctx.i
	// Opponent nbits if extensible set.
	ahead := uint16(0)

	if t.extensible {
		if ctx.isEncode {
			// Encode ahead flag if extensible.
			t.EncodeExtensibleAhead(ctx)
		} else {
			// Decode ahead flag if extensible.
			ahead = t.DecodeExtensibleAhead(ctx)
		}
	}

	// Process fields.
	for _, fieldDescriptor := range t.fieldDescriptors {
		fieldDescriptor.Process(ctx, di, accessor)
	}

	// Skip redundant bits post decoding.
	if t.extensible && !ctx.isEncode {
		// Skip redundant bits.
		ito := i + int(ahead)
		if ito >= ctx.i {
			ctx.i = ito
		}
	}
}

// EncodeExtensibleAhead encode the message number of bits as the ahead flag to
// current bit encoding stream.
func (t *MessageProcessor) EncodeExtensibleAhead(ctx *ProcessContext) {
	// Safe to cast:
	// the nbits of a message always <= 65535.
	data := uint16(t.nbits)
	accessor := &Uint16Accessor{data}
	di := NewDataIndexer(1)
	processBaseType(16, ctx, di, accessor)
}

// DecodeExtensibleAhead decode the ahead flag as the nbits of this message from
// current decoding stream.
func (t *MessageProcessor) DecodeExtensibleAhead(ctx *ProcessContext) uint16 {
	accessor := &Uint16Accessor{}
	di := NewDataIndexer(1)
	processBaseType(16, ctx, di, accessor)
	return accessor.data
}

// processBaseType process encoding and decoding on a base type.
func processBaseType(nbits int, ctx *ProcessContext, di *DataIndexer, accessor Accessor) {
	for j := 0; j < nbits; {
		// Gets number of bits to copy (0~8).
		c := getNbitsToCopy(ctx.i, j, nbits)
		// Process bits copy inside a byte.
		processSingleByte(ctx, di, accessor, j, c)
		// Maintain trackers.
		ctx.i += c
		j += c
	}
}

// processSingleByte dispatch process to encodeSingleByte and decodeSingleByte.
func processSingleByte(ctx *ProcessContext, di *DataIndexer, accessor Accessor, j, c int) {
	if ctx.isEncode {
		encodeSingleByte(ctx, di, accessor, j, c)
	} else {
		decodeSingleByte(ctx, di, accessor, j, c)
	}
}

// encodeSingleByte encode number of c bits from data to given buffer s.
// Where the data is lookedup by data indexer di from data container accessor.
// And the buffer s is given in ProcessContext ctx.
func encodeSingleByte(ctx *ProcessContext, di *DataIndexer, accessor Accessor, j, c int) {
	i := ctx.i

	// Number of bits to shift right to obtain byte from accessor.
	rshift := int(j/8) * 8
	b := accessor.BpGetByte(di, rshift)

	shift := (j % 8) - (i % 8)
	mask := byte(getMask(i%8, c)) // safe cast: i%8 and c always in [0,8]
	// shift and then take mask to get bits to copy.
	d := smartShift(b, shift) & mask

	// Copy bits to buffer s.
	ctx.s[int(i/8)] |= d
}

// decodeSingleByte decode number of c bits from buffer s to target data.
// Where the data is lookedup by data indexer di from data container accessor.
// And the buffer s is given in ProcessContext ctx.
// Byte decoding is finally done by accessor's generated function BpSetByte.
func decodeSingleByte(ctx *ProcessContext, di *DataIndexer, accessor Accessor, j, c int) {
	i := ctx.i

	b := ctx.s[int(i/8)]
	shift := (i % 8) - (j % 8)
	mask := byte(getMask(j%8, c)) // safe cat: j%8 and c always in [0,8]
	// shift and then take mask to get bits to copy.
	d := smartShift(b, shift) & mask

	// Number of bits to shift left to set byte to accessor.
	lshift := int(j/8) * 8
	accessor.BpSetByte(di, lshift, d)
}

// Returns the number of bits to copy during a single byte process.
// Argument i, j, n:
//
//	i is the number of the total bits processed.
//	j is the number of bits processed on current base type.
//	n is the number of bits current base type occupy.
//
// The returned value always in [0, 8].
func getNbitsToCopy(i, j, n int) int {
	return min(min(n-j, 8-(j%8)), 8-(i%8))
}

// getMask returns the mask value to copy bits inside a single byte.
// The argument k is the start bit index in the byte, argument c is the number
// of bits to copy.
//
// Examples of returned mask:
//
//	Returns                Arguments
//	00001111               k=0, c=4
//	01111100               k=2, c=5
//	00111100               k=2, c=4
func getMask(k, c int) int {
	if k == 0 {
		return (1 << c) - 1

	}
	return (1 << ((k + 1 + c) - 1)) - (1 << ((k + 1) - 1))
}

// Returns the smaller one of given two integers.
func min(a, b int) int {
	if a < b {
		return a

	}
	return b
}

// BpSmartShift shifts given byte n by k bits.
// If k is larger than 0, performs a right shift, otherwise left.
func smartShift(n byte, k int) byte {
	if k > 0 {
		return n >> k
	}
	if k < 0 {
		return n << (0 - k)
	}
	return n
}

// Returns a byte from a bool.
func Bool2byte(b bool) byte {
	if b {
		return 1
	}
	return 0
}

// Returns a bool from a byte.
func Byte2bool(b byte) bool {
	if b > 0 {
		return true
	}
	return false
}
