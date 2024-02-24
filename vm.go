package main

import (
	"bufio"
	"fmt"
	"io"
	"reflect"
)

type OpCode int

const (
	Op_ListCommands OpCode = iota
	Op_Call
	Op_Ld_i64
	Op_Ld_String
	Op_Ld_F64
	Op_Ld_U8_Array
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

func read_sleb64(s *bufio.Reader) int64 {
	var value int64 = 0
	var shift uint32 = 0
	var chunk byte
	var err error

	for {
		chunk, err = s.ReadByte()

		if err != nil {
			panic(err.Error())
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

	return value
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

	default:
		panic(fmt.Sprintf("unsupported type! %v", obj))
	}
}

func read_from_stream(reader *bufio.Reader) interface{} {
	t, e := reader.ReadByte()
	if e != nil {
		panic(e.Error())
	}
	t2 := Type(t)
	switch t2 {
	case Type_I64:
		v := read_sleb64(reader)
		return v
	case Type_String:
		count := int(read_sleb64(reader))
		arr := make([]byte, count)
		cnt, err := reader.Read(arr)
		if err != nil {
			panic(err.Error())
		}
		if cnt != count {
			panic("unable to read expected number of bytes")
		}
		return string(arr)
	case Type_Error:
		str, e := read_from_stream(reader).(string)
		if !e {
			panic("unexpected type")
		}
		return fmt.Errorf(str)
	}
	panic(fmt.Sprintf("cannot read type: %v", t2))
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

func eval_stream(read_stream io.Reader, writer_stream io.Writer) {
	commands := []Command{AddCommand, SubCommand}
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
			op := read_sleb64(reader)
			stack.Push(op)
		case Op_Dup:
			stack.Push(stack.Peek())
		case Op_Pop:
			stack.Pop()
		case Op_Call:
			op := read_sleb64(reader)
			cmd := commands[op]
			arglen := len(cmd.Arguments)
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
