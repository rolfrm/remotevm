package remotevm

import (
	"bufio"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"reflect"
	"time"

	"github.com/quic-go/quic-go"
)

type OpCode int

const (
	Op_ListCommands OpCode = iota
	// call [id] Calls an
	Op_Call
	// ld [i64 sleb] loads an int64
	Op_Ld_i64
	// ld [generic] loads anything
	Op_Ld
	Op_Pop
	Op_Dup
	Op_Return
	Op_Forward
)

type Type byte

const (
	Type_I64 Type = iota
	Type_F64
	Type_String
	Type_U8_Array
	Type_Type_Array
	Type_Object
	Type_Command
	Type_Command_Array
	Type_Nothing
	Type_Error
)

func (t Type) String() string {
	switch t {
	case Type_I64:
		return "Type_I64"
	case Type_F64:
		return "Type_F64"
	case Type_String:
		return "Type_String"
	case Type_U8_Array:
		return "Type_U8_Array"
	case Type_Type_Array:
		return "Type_Type_Array"
	case Type_Object:
		return "Type_Object"
	case Type_Command:
		return "Type_Command"
	case Type_Command_Array:
		return "Type_Command_Array"
	case Type_Nothing:
		return "Type_Nothing"
	case Type_Error:
		return "Type_Error"
	default:
		return fmt.Sprintf("Unknown Type: %d", t)
	}
}

type Command struct {
	id        int64
	Name      string
	Arguments []Type
	Func      interface{}
}

type Server struct {
	Commands []Command
	End      chan bool
	KeyFile  string
	CertFile string
	Address  string
}

func ServerNew() *Server {
	return &Server{Commands: make([]Command, 0), End: make(chan bool), KeyFile: "server.key", CertFile: "server.crt", Address: "localhost:42424"}
}

func writer_i64_sleb(inValue int64, w *bufio.Writer) {
	value := inValue
	for {
		bits := byte(value & 0b01111111)
		sign := byte(value & 0b01000000)
		next := value >> 7

		if (next == 0 && sign == 0) || (sign > 0 && next == -1) {
			w.WriteByte(bits)
			break
		} else {
			w.WriteByte(bits | 0b10000000)
			value = next
		}
	}
}

func read_sleb64(s *bufio.Reader) (int64, error) {
	var value int64 = 0
	var shift uint32 = 0
	var chunk byte
	var err error

	for {
		chunk, err = s.ReadByte()

		if err != nil {
			return 0, err
		}
		value |= int64(chunk&0x7f) << shift
		shift += 7
		if chunk < 128 {
			break
		}
	}

	if shift < 64 && (chunk&0x40) > 0 {
		value |= int64(uint64(^uint64(0)) << shift)
	}

	return value, nil
}

func write_to_stream(value interface{}, writer *bufio.Writer) {

	switch obj := value.(type) {
	case Command:
		writer.WriteByte(byte(Type_Command))
		write_to_stream(obj.id, writer)
		write_to_stream(obj.Name, writer)
		write_to_stream(obj.Arguments, writer)
	case []Command:
		writer.WriteByte(byte(Type_Command_Array))
		writer_i64_sleb(int64(len(obj)), writer)
		for i := range obj {
			write_to_stream(obj[i], writer)
		}
	case string:
		writer.WriteByte(byte(Type_String))
		bytes := []byte(obj)
		writer_i64_sleb(int64(len(bytes)), writer)
		writer.Write(bytes)
	case int64:
		writer.WriteByte(byte(Type_I64))
		writer_i64_sleb(obj, writer)
	case float64:
		writer.WriteByte(byte(Type_F64))
		bits := math.Float64bits(obj)
		bytes := make([]byte, 8)
		binary.BigEndian.PutUint64(bytes, bits)
		writer.Write(bytes)
	case []Type:
		writer.WriteByte(byte(Type_Type_Array))
		writer.WriteByte(byte(len(obj)))
		for i := range obj {
			writer.WriteByte(byte(obj[i]))
		}
	case error:
		writer.WriteByte(byte(Type_Error))
		write_to_stream(obj.Error(), writer)
	case nil:
		writer.WriteByte(byte(Type_Nothing))
	case []byte:
		writer.WriteByte(byte(Type_U8_Array))
		writer_i64_sleb(int64(len(obj)), writer)
		writer.Write(obj)

	default:
		panic(fmt.Sprintf("unsupported type! %v", obj))
	}
}

func read_from_stream(reader *bufio.Reader) (interface{}, error) {
	t, e := reader.ReadByte()
	if e != nil {
		return nil, e
	}
	t2 := Type(t)
	switch t2 {
	case Type_I64:
		return read_sleb64(reader)
	case Type_F64:
		bytes := make([]byte, 8)
		r, e := reader.Read(bytes)
		if e != nil {
			return nil, e
		}
		if r != 8 {
			return nil, fmt.Errorf("expected 8 bytes read")
		}

		//binary.BigEndian.PutUint64(bytes, bits)
		bits := binary.BigEndian.Uint64(bytes)
		return math.Float64frombits(bits), nil
	case Type_U8_Array:
		count0, e := read_sleb64(reader)
		if e != nil {
			return nil, e
		}
		count := int(count0)
		arr := make([]byte, count)
		i, err := reader.Read(arr)
		if err != nil {
			return nil, err
		}
		if i != count {
			return nil, fmt.Errorf("expected to read %v bytes", count)
		}
		return arr, nil

	case Type_String:
		count0, e := read_sleb64(reader)
		if e != nil {
			return nil, e
		}
		count := int(count0)
		arr := make([]byte, count)
		readCnt, err := reader.Read(arr)
		if err != nil {
			return nil, err
		}
		if count != readCnt {
			return nil, fmt.Errorf("unable to read expected number of bytes")
		}
		return string(arr), nil
	case Type_Error:
		str, e := read_from_stream(reader)
		if e != nil {
			return nil, e
		}
		if str2, ok := str.(string); ok {
			return fmt.Errorf(str2), nil
		}
		return nil, fmt.Errorf("unexpected object read")

	}
	return nil, fmt.Errorf("cannot read type: %v", t2)
}

func dynamicInvoke(function interface{}, args []interface{}) (result []interface{}, err error) {
	defer func() {
		if err2 := recover(); err2 != nil {
			err = fmt.Errorf("%v", err2)
		}
	}()
	// Get the reflect.Value of the function
	funcValue := reflect.ValueOf(function)

	// Make sure the function is a valid function
	if funcValue.Kind() != reflect.Func {
		err = fmt.Errorf("provided value is not a function")
		return
	}

	// Prepare the arguments
	var inputValues []reflect.Value
	for _, arg := range args {
		inputValues = append(inputValues, reflect.ValueOf(arg))
	}

	// Call the function with the provided arguments
	resultValues := funcValue.Call(inputValues)

	// Convert the result values to a slice of interfaces
	for _, val := range resultValues {
		result = append(result, val.Interface())
	}

	return
}

func eval_stream(commands []Command, read_stream io.Reader, writer_stream io.Writer) {
	reader := bufio.NewReader(read_stream)
	writer := bufio.NewWriter(writer_stream)
	stack := Stack{}
	for {
		b, e := reader.ReadByte()
		if e != nil {

			break
		}

		switch OpCode(b) {
		case Op_ListCommands:
			stack.Push(commands)
		case Op_Return:
			val := stack.Pop()
			write_to_stream(val, writer)
			writer.Flush()
		case Op_Ld_i64:
			op, err := read_sleb64(reader)
			if err != nil {
				write_to_stream(err, writer)
				writer.Flush()
				return
			}
			stack.Push(op)
		case Op_Ld:
			op, err := read_from_stream(reader)
			if err != nil {
				write_to_stream(err, writer)
				writer.Flush()
				return
			}
			stack.Push(op)
		case Op_Dup:
			stack.Push(stack.Peek())
		case Op_Pop:
			stack.Pop()
		case Op_Call:
			op, err := read_sleb64(reader)
			if err != nil {
				write_to_stream(err, writer)
				writer.Flush()
				return
			}
			if len(commands) < int(op) {
				write_to_stream(fmt.Errorf("no such opcode: %v", op), writer)
				return
			}
			cmd := commands[op]
			arglen := len(cmd.Arguments)
			if arglen > len(stack.items) {
				write_to_stream(fmt.Errorf("stack exhausted"), writer)
				return
			}
			args := make([]interface{}, arglen)
			for i := 0; i < arglen; i++ {
				args[i] = stack.Pop()
			}

			result, e := dynamicInvoke(cmd.Func, args)
			if e != nil {
				write_to_stream(e, writer)
				break
			}
			for x := range result {
				stack.Push(result[x])

			}
		case Op_Forward:
			write_to_stream(fmt.Errorf("not implemented"), writer)
		}

	}

}

type emptyCtx struct{}

func (emptyCtx) Deadline() (deadline time.Time, ok bool) {
	return
}

func (emptyCtx) Done() <-chan struct{} {
	return nil
}

func (emptyCtx) Err() error {
	return nil
}

func (emptyCtx) Value(key any) any {
	return nil
}

func (s *Server) go_con_quic(con quic.Connection) {
	fmt.Println("Got connection to client!")
	str, err := con.AcceptStream(con.Context())
	if err != nil {
		panic(err.Error())
	}
	eval_stream(s.Commands, str, str)
}

func (s *Server) Serve() {
	keyFile := s.KeyFile
	certFile := s.CertFile
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	tlscfg := tls.Config{
		InsecureSkipVerify: true,
		Certificates:       []tls.Certificate{cert},
	}

	listener, err := quic.ListenAddr(s.Address, &tlscfg, nil)
	if err != nil {
		panic(err.Error())
		return
	}

	defer listener.Close()
	go func() {
		<-s.End
		listener.Close()
	}()
	x := emptyCtx{}
	for {
		fmt.Println("Listening for connection")
		con, err := listener.Accept(&x)
		if err != nil {
			panic(err.Error())
		}
		fmt.Println("Got connection")
		go s.go_con_quic(con)

	}

}

type Client struct {
	con quic.Connection
}

type ClientStream struct {
	Stream    quic.Stream
	outBuffer *bufio.Writer
}

func (str *ClientStream) Write(args ...interface{}) {
	for _, v := range args {
		write_to_stream(v, str.outBuffer)
	}
}

func (cli *Client) OpenStream() (*ClientStream, error) {
	str, e := cli.con.OpenStream()
	if e != nil {
		return nil, e
	}
	return &ClientStream{Stream: str}, nil
}

func NewClient(addr string) Client {
	ctx := emptyCtx{}

	tlscfg := tls.Config{InsecureSkipVerify: true}
	quiccfg := quic.Config{}

	con, err := quic.DialAddr(&ctx, addr, &tlscfg, &quiccfg)

	if err != nil {
		panic(err.Error())
	}

	return Client{con}
}
