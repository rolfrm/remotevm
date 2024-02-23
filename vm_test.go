package main

import (
	"bufio"
	"bytes"
	"fmt"
	"testing"
)

type TestStream struct {
	bytes.Buffer
}

func (t *TestStream) Close() error {
	return nil
}

func TestWriteToStream(t *testing.T) {
	testCases := []struct {
		name  string
		value interface{}
		want  []byte
	}{
		{
			name:  "Command",
			value: AddCommand,
			want:  []byte{Type_Command, 0, 0, Type_String, 1, '+', Type_Type_Array, 2, Type_I64, Type_I64},
		},
		{
			name:  "String",
			value: "Hello, Go!",
			want:  []byte{Type_String, byte(len("Hello, Go!")), 'H', 'e', 'l', 'l', 'o', ',', ' ', 'G', 'o', '!'},
		},
		{
			name:  "Int64",
			value: int64(42),
			want:  []byte{Type_I64, 42},
		},
		{
			name:  "TypeArray",
			value: []Type{Type_F64, Type_String},
			want:  []byte{Type_Type_Array, 2, Type_F64, Type_String},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			writer := bufio.NewWriter(&buf)
			write_to_stream(tc.value, writer)
			writer.Flush()

			got := buf.Bytes()
			if !bytes.Equal(got, tc.want) {
				t.Errorf("got: %v, want: %v", got, tc.want)
			}
		})
	}
}
func TestReadSLEB64(t *testing.T) {
	// Example test for read_sleb64
	stream := &TestStream{}
	writer := bufio.NewWriter(stream)
	writer_i64_sleb(int64(42), writer)
	writer.Flush()

	reader := bufio.NewReader(stream)
	got := read_sleb64(reader)
	want := int64(42)

	if got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}

	// Add more tests as needed
}

func TestEvalStream(t *testing.T) {
	var b bytes.Buffer
	var b2 bytes.Buffer

	writer := bufio.NewWriter(&b)
	writer2 := bufio.NewWriter(&b2)

	// Write OpCode and corresponding data to buffer
	writer.WriteByte(byte(Op_Ld_i64))
	writer_i64_sleb(222333, writer)
	writer.WriteByte(byte(Op_Return))
	writer.Flush()

	eval_stream(&b, writer2)
	writer2.Flush()

	// Now the buffer should contain the returned value

	buf2 := bufio.NewReader(&b2)
	buf2.ReadByte()
	result := read_sleb64(buf2)

	if result != 222333 {
		t.Errorf("eval_stream was incorrect, got: %d, want: %d.", result, 222333)
	}
}

func TestEvalStream2(t *testing.T) {
	var b bytes.Buffer
	var b2 bytes.Buffer

	writer := bufio.NewWriter(&b)
	writer2 := bufio.NewWriter(&b2)

	// Write OpCode and corresponding data to buffer
	writer.WriteByte(byte(Op_ListCommands))
	writer.WriteByte(byte(Op_Return))
	writer.Flush()

	eval_stream(&b, writer2)
	writer2.Flush()
	fmt.Printf(" >>  %v", b2.Bytes())

}

func TestEvalStream3(t *testing.T) {
	var b bytes.Buffer
	var b2 bytes.Buffer

	writer := bufio.NewWriter(&b)
	writer2 := bufio.NewWriter(&b2)

	// Write OpCode and corresponding data to buffer
	writer.WriteByte(byte(Op_Ld_i64))
	writer_i64_sleb(-10, writer)
	writer.WriteByte(byte(Op_Ld_i64))
	writer_i64_sleb(120, writer)
	writer.WriteByte(byte(Op_Call))
	writer.WriteByte(0)
	writer.WriteByte(byte(Op_Dup))
	writer.WriteByte(byte(Op_Call))
	writer.WriteByte(0)
	writer.WriteByte(byte(Op_Return))
	writer.Flush()

	eval_stream(&b, writer2)
	writer2.Flush()

	buf2 := bufio.NewReader(&b2)
	result := read_from_stream(buf2)

	if result != int64(220) {
		t.Errorf("eval_stream was incorrect, got: %d, want: %d.", result, 110)
	}
}

func TestThroughQuic(t *testing.T) {
	end := make(chan bool)
	go serve_quic(end)
	defer func() { end <- true }()

	cli := new_client()
	str, err := cli.OpenStream()
	if err != nil {
		t.Error(err.Error())
	}
	w := bufio.NewWriter(str)
	r := bufio.NewReader(str)
	w.WriteByte(byte(Op_Ld_i64))
	writer_i64_sleb(112233, w)
	w.WriteByte(byte(Op_Dup))
	w.WriteByte(byte(Op_Call))
	w.WriteByte(byte(0))
	w.WriteByte(byte(Op_Return))
	w.Flush()

	result := read_from_stream(r)
	if err != nil {
		t.Error(err.Error())
	}
	if result != int64(224466) {
		t.Errorf("unexpected bytes from stream %v", result)
	}
}
