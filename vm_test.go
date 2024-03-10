package remotevm

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

func Add(a int64, b int64) int64 {
	return a + b
}

var AddCommand = Command{
	id:        0,
	Name:      "+",
	Arguments: []Type{Type_I64, Type_I64},
	Func:      Add,
}

func Sub(a int64, b int64) int64 {
	return a - b
}

var SubCommand = Command{
	id:        1,
	Name:      "-",
	Arguments: []Type{Type_I64, Type_I64},
	Func:      Sub,
}

func Concat(context *StreamContext, a string, b string) string {
	return fmt.Sprintf("%s%s", a, b)
}

var ConcatCommand = Command{
	id:           2,
	Name:         "..",
	Arguments:    []Type{Type_String, Type_String},
	Func:         Concat,
	NeedsContext: true,
}

var commands []Command = []Command{AddCommand, SubCommand, ConcatCommand}

func TestWriteToStream(t *testing.T) {
	testCases := []struct {
		name  string
		value interface{}
		want  []byte
	}{
		{
			name:  "Command",
			value: AddCommand,
			want:  []byte{byte(Type_Command), 0, 0, byte(Type_String), 1, '+', byte(Type_Type_Array), 2, byte(Type_I64), byte(Type_I64)},
		},
		{
			name:  "String",
			value: "Hello, Go!",
			want:  []byte{byte(Type_String), byte(len("Hello, Go!")), 'H', 'e', 'l', 'l', 'o', ',', ' ', 'G', 'o', '!'},
		},
		{
			name:  "Int64",
			value: int64(42),
			want:  []byte{byte(Type_I64), 42},
		},
		{
			name:  "TypeArray",
			value: []Type{Type_F64, Type_String},
			want:  []byte{byte(Type_Type_Array), 2, byte(Type_F64), byte(Type_String)},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			writer := bufio.NewWriter(&buf)
			writeToStream(tc.value, writer)
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
	got, _ := read_sleb64(reader)
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

	EvalStream(commands, &b, writer2)
	writer2.Flush()

	// Now the buffer should contain the returned value

	buf2 := bufio.NewReader(&b2)
	buf2.ReadByte()
	result, _ := read_sleb64(buf2)

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

	EvalStream(commands, &b, writer2)
	writer2.Flush()
	fmt.Printf(" >>  %v", b2.Bytes())

}

func load_test_add(writer *bufio.Writer) {
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
}
func load_test_sub(writer *bufio.Writer) {
	writer.WriteByte(byte(Op_Ld_i64))
	writer_i64_sleb(50, writer)
	writer.WriteByte(byte(Op_Ld_i64))
	writer_i64_sleb(120, writer)
	writer.WriteByte(byte(Op_Call))
	writer.WriteByte(1)
	writer.WriteByte(byte(Op_Return))
	writer.Flush()
}
func load_test_concat_call(writer *bufio.Writer) {
	writer.WriteByte(byte(Op_Ld))
	writeToStream("456", writer)
	writer.WriteByte(byte(Op_Ld))
	writeToStream("123", writer)
	writer.WriteByte(byte(Op_Call))
	writer.WriteByte(2)
	writer.WriteByte(byte(Op_Return))
	writer.Flush()
}
func load_test_error_call(writer *bufio.Writer) {
	writer.WriteByte(byte(Op_Ld))
	writeToStream("456", writer)
	writer.WriteByte(byte(Op_Ld))
	writeToStream("123", writer)

	writer.WriteByte(byte(Op_Call))
	writer.WriteByte(0)
	writer.WriteByte(byte(Op_Return))
	writer.Flush()
}
func load_test_err_call2(writer *bufio.Writer) {
	writer.WriteByte(byte(Op_Ld))
	writer_i64_sleb(50, writer)
	writer.WriteByte(byte(Op_Call))
	writer.Flush()
}
func load_test_err_call3(writer *bufio.Writer) {
	writer.WriteByte(byte(Op_Ld))
	writeToStream(int64(55), writer)
	writer.WriteByte(byte(Op_Call))
	writer.WriteByte(byte(0))
	writer.Flush()
}
func load_test_err_call4(writer *bufio.Writer) {
	writer.WriteByte(byte(Op_Call))
	writer.WriteByte(byte(len(commands)))
	writer.Flush()
}
func load_test_write_read_bytes(thing interface{}, writer *bufio.Writer) {
	writer.WriteByte(byte(Op_Ld))
	writeToStream(thing, writer)
	writer.WriteByte(byte(Op_Return))
	writer.Flush()
}

func TestEvalStream3(t *testing.T) {

	numbers := make([]byte, 10)
	for i := range numbers {
		numbers[i] = byte(i)
	}

	testCases := []struct {
		name     string
		function func(*bufio.Writer)
		want     interface{}
	}{
		{name: "AddTest", function: load_test_add, want: int64(220)},
		{name: "SubTest", function: load_test_sub, want: int64(70)},
		{name: "ConcatTest", function: load_test_concat_call, want: "123456"},
		{name: "WriteReadf64", function: func(w *bufio.Writer) { load_test_write_read_bytes(3.14, w) }, want: 3.14},

		{name: "WriteReadBytes", function: func(w *bufio.Writer) { load_test_write_read_bytes(numbers, w) }, want: numbers},
		{name: "ErrCall", function: load_test_error_call, want: fmt.Errorf("reflect: Call using string as type int64")},
		{name: "ErrCall2", function: load_test_err_call2, want: fmt.Errorf("cannot read type: Unknown Type: 50")},
		{name: "ErrCall3", function: load_test_err_call3, want: fmt.Errorf("stack exhausted")},
		{name: "ErrCall4", function: load_test_err_call4, want: fmt.Errorf("no such opcode: 3")},
	}
	for _, tc := range testCases {
		fmt.Printf("test case: %s\n", tc.name)
		var b bytes.Buffer
		var b2 bytes.Buffer

		writer := bufio.NewWriter(&b)
		writer2 := bufio.NewWriter(&b2)
		tc.function(writer)
		EvalStream(commands, &b, writer2)
		writer2.Flush()

		buf2 := bufio.NewReader(&b2)
		result, _ := readFromStream(buf2)

		wantString := fmt.Sprintf("%v", tc.want)
		resultString := fmt.Sprintf("%v", result)
		fmt.Printf(" %v   %v\n", wantString, resultString)

		if wantString != resultString {
			t.Errorf("eval_stream3 was incorrect, got: %v, want: %v.", result, tc.want)
		}
	}

}

func TestThroughQuic(t *testing.T) {
	server := ServerNew()
	server.Commands = commands
	go server.Serve()
	defer func() { server.End <- true }()

	cli := NewClient(server.Address)
	str, err := cli.OpenStream()
	if err != nil {
		t.Error(err.Error())
	}
	str.Write(Op_Ld, int64(112233), Op_Dup, Op_Call, byte(0), Op_Return)

	result, err := str.Read()
	if err != nil {
		t.Error(err.Error())
	}
	if result != int64(224466) {
		t.Errorf("unexpected bytes from stream %v", result)
	}
}
