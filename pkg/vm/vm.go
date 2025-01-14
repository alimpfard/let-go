/*
 * Copyright (c) 2021 Marcin Gasperowicz <xnooga@gmail.com>
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated
 * documentation files (the "Software"), to deal in the Software without restriction, including without limitation the
 * rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit
 * persons to whom the Software is furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all copies or substantial portions of the
 * Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE
 * WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
 * COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR
 * OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
 */

package vm

import (
	"encoding/binary"
	"fmt"
)

// Opcodes
const (
	OPNOP uint8 = iota // do nothing

	OPLDC // load constant LDC (index int32)
	OPLDA // load argument LDA (index int32)

	OPINV // invoke function
	OPRET // return from function

	OPBRT // branch if truthy BRT (offset int32)
	OPBRF // branch if falsy BRF (offset int32)
	OPJMP // jump by offset JMP (offset int32)

	OPPOP // pop value from the stack and discard it
	OPPON // save top and pop n elements from the stack PON (n int32)
	OPDPN // duplicate nth value from the stack OPN (n int32)

	OPSTV // set var
	OPLDV // push var root

	OPLDK // load closed over LDK (index int32)
	OPPAK // push closed over value to a closure
)

func OpcodeToString(op uint8) string {
	ops := []string{"NOP", "LDC", "LDA", "INV", "RET", "BRT", "BRF", "JMP", "POP", "PON", "DPN", "STV", "LDV", "LDK", "PAK"}
	if int(op) < len(ops) {
		return ops[op]
	}
	return "???"
}

// CodeChunk holds bytecode and provides facilities for reading and writing it
type CodeChunk struct {
	maxStack int
	consts   *[]Value
	code     []uint8
	length   int
}

func NewCodeChunk(consts *[]Value) *CodeChunk {
	return &CodeChunk{
		consts: consts,
		code:   []uint8{},
		length: 0,
	}
}

func (c *CodeChunk) Debug() {
	fmt.Println("consts:")
	consts := *c.consts
	for i := range consts {
		fmt.Println("  [", i, "] =", consts[i])
	}
	fmt.Println("code:")
	i := 0
	for i < len(c.code) {
		op, _ := c.Get(i)
		switch op {
		case OPLDC, OPLDA, OPBRT, OPBRF, OPJMP, OPPON, OPDPN, OPINV, OPLDK:
			arg, _ := c.Get32(i + 1)
			fmt.Println("  ", i, ":", OpcodeToString(op), arg)
			i += 5
		default:
			fmt.Println("  ", i, ":", OpcodeToString(op))
			i++
		}
	}
}

func (c *CodeChunk) Length() int {
	return c.length
}

func (c *CodeChunk) Append(insts ...uint8) {
	c.code = append(c.code, insts...)
	c.length = len(c.code)
}

func (c *CodeChunk) Append32(val int) {
	n := make([]uint8, 4)
	binary.LittleEndian.PutUint32(n, uint32(val))
	c.code = append(c.code, n...)
	c.length = len(c.code)
}

func (c *CodeChunk) AppendChunk(o *CodeChunk) {
	if o.maxStack > c.maxStack {
		c.maxStack = o.maxStack
	}
	c.code = append(c.code, o.code...)
	c.length += len(o.code)
}

func (c *CodeChunk) Get(idx int) (uint8, error) {
	if idx >= c.length {
		return 0, NewExecutionError("bytecode fetch out of bounds")
	}
	return c.code[idx], nil
}

func (c *CodeChunk) Get32(idx int) (int, error) {
	if idx >= c.length || idx+4 > c.length {
		return 0, NewExecutionError("bytecode wide fetch out of bounds")
	}
	return int(binary.LittleEndian.Uint32(c.code[idx:])), nil
}

func (c *CodeChunk) Update32(address int, value int) {
	binary.LittleEndian.PutUint32(c.code[address:address+4], uint32(value))
}

func (c *CodeChunk) SetMaxStack(max int) {
	c.maxStack = max
}

const defaultStackSize = 32

// Frame is a single interpreter context
type Frame struct {
	stack       []Value
	args        []Value
	closedOvers []Value
	argc        int
	consts      []Value
	constsc     int
	code        *CodeChunk
	ip          int
	sp          int
}

func NewFrame(code *CodeChunk, args []Value) *Frame {
	return &Frame{
		stack:   make([]Value, code.maxStack),
		args:    args,
		argc:    len(args),
		consts:  *code.consts,
		constsc: len(*code.consts),
		code:    code,
		ip:      0,
		sp:      0,
	}
}

func (f *Frame) Push(v Value) error {
	if f.sp >= defaultStackSize-1 {
		return NewExecutionError("stack overflow")
	}
	f.stack[f.sp] = v
	f.sp++
	return nil
}

func (f *Frame) Pop() (Value, error) {
	if f.sp == 0 {
		return NIL, NewExecutionError("stack underflow")
	}
	f.sp--
	v := f.stack[f.sp]
	//f.stack[f.sp] = nil
	return v, nil
}

func (f *Frame) Nth(n int) (Value, error) {
	i := f.sp - 1 - n
	if i < 0 {
		return NIL, NewExecutionError("Nth: stack underflow")
	}
	return f.stack[i], nil
}

func (f *Frame) Mult(start int, count int) ([]Value, error) {
	if count < 0 {
		return nil, NewExecutionError("Mult: count 0 or negative")
	}
	i := f.sp - start
	if i-count < 0 {
		return nil, NewExecutionError("Mult: stack underflow")
	}
	return f.stack[i-count : i], nil
}

func (f *Frame) Drop(n int) error {
	top := f.sp - 1
	if top < 0 {
		return NewExecutionError("Drop: stack underflow")
	}
	f.sp -= n
	if f.sp < 0 {
		return NewExecutionError("Drop: stack underflow")
	}
	// for i := top; i >= f.sp; i-- {
	// 	f.stack[i] = nil
	// }
	return nil
}

func (f *Frame) Run() (Value, error) {
	for {
		inst, _ := f.code.Get(f.ip)
		//	fmt.Println("exec", f.ip, OpcodeToString(inst))
		switch inst {
		case OPNOP:
			f.ip++

		case OPLDC:
			idx, err := f.code.Get32(f.ip + 1)
			if err != nil {
				return NIL, NewExecutionError("const push failed").Wrap(err)
			}
			if idx >= f.constsc {
				return NIL, NewExecutionError("const lookup out of bounds")
			}
			err = f.Push(f.consts[idx])
			if err != nil {
				return NIL, NewExecutionError("const push failed").Wrap(err)
			}
			f.ip += 5

		case OPLDA:
			idx, err := f.code.Get32(f.ip + 1)
			if err != nil {
				return NIL, NewExecutionError("get argument index failed").Wrap(err)
			}
			if idx >= f.argc {
				return NIL, NewExecutionError("argument lookup out of bounds")
			}
			err = f.Push(f.args[idx])
			if err != nil {
				return NIL, NewExecutionError("argument push failed").Wrap(err)
			}
			f.ip += 5

		case OPRET:
			v, err := f.Pop()
			if err != nil {
				return NIL, NewExecutionError("return failed").Wrap(err)
			}
			return v, nil

		case OPINV:
			arity, err := f.code.Get32(f.ip + 1)
			if err != nil {
				return NIL, NewExecutionError("INV arg count").Wrap(err)
			}
			fraw, err := f.Nth(arity)
			if err != nil {
				return NIL, NewExecutionError("invoke instruction failed").Wrap(err)
			}
			fn, ok := fraw.(Fn)
			if !ok {
				return NIL, NewTypeError(fraw, "is not a function", nil)
			}
			a, err := f.Mult(0, arity)
			if err != nil {
				return NIL, NewExecutionError("popping arguments failed").Wrap(err)
			}
			out := fn.Invoke(a)
			err = f.Drop(arity + 1)
			if err != nil {
				return NIL, NewExecutionError("cleaning stack after call").Wrap(err)
			}
			err = f.Push(out)
			if err != nil {
				return NIL, NewExecutionError("pushing return value failed").Wrap(err)
			}
			f.ip += 5

		case OPBRT:
			offset, err := f.code.Get32(f.ip + 1)
			if err != nil {
				return NIL, NewExecutionError("BRT offset").Wrap(err)
			}
			v, err := f.Pop()
			if err != nil {
				return NIL, NewExecutionError("BRT pop condition").Wrap(err)
			}
			if !IsTruthy(v) {
				f.ip += 5
				continue
			}
			f.ip += offset

		case OPBRF:
			offset, err := f.code.Get32(f.ip + 1)
			if err != nil {
				return NIL, NewExecutionError("BRT offset").Wrap(err)
			}
			v, err := f.Pop()
			if err != nil {
				return NIL, NewExecutionError("BRT pop condition").Wrap(err)
			}
			if IsTruthy(v) {
				f.ip += 5
				continue
			}
			f.ip += offset

		case OPJMP:
			offset, err := f.code.Get32(f.ip + 1)
			if err != nil {
				return NIL, NewExecutionError("JMP offset").Wrap(err)
			}
			f.ip += offset

		case OPPOP:
			_, err := f.Pop()
			if err != nil {
				return NIL, NewExecutionError("POP failed").Wrap(err)
			}
			f.ip++

		case OPPON:
			v, err := f.Pop()
			if err != nil {
				return NIL, NewExecutionError("PON top value").Wrap(err)
			}
			num, err := f.code.Get32(f.ip + 1)
			if err != nil {
				return NIL, NewExecutionError("PON get argument").Wrap(err)
			}
			err = f.Drop(num)
			if err != nil {
				return NIL, NewExecutionError("PON drop").Wrap(err)
			}
			err = f.Push(v)
			if err != nil {
				return NIL, NewExecutionError("PON push").Wrap(err)
			}
			f.ip += 5

		case OPDPN:
			num, err := f.code.Get32(f.ip + 1)
			if err != nil {
				return NIL, NewExecutionError("DPN get argument").Wrap(err)
			}
			val, err := f.Nth(num)
			if err != nil {
				return NIL, NewExecutionError("DPN get nth").Wrap(err)
			}
			err = f.Push(val)
			if err != nil {
				return NIL, NewExecutionError("DPN push").Wrap(err)
			}
			f.ip += 5

		case OPSTV:
			val, err := f.Pop()
			if err != nil {
				return NIL, NewExecutionError("STV pop value failed").Wrap(err)
			}
			varr, err := f.Pop()
			if err != nil {
				return NIL, NewExecutionError("STV pop var failed").Wrap(err)
			}
			varrd, ok := varr.(*Var)
			if !ok {
				return NIL, NewExecutionError("STV invalid Var").Wrap(err)
			}
			varrd.SetRoot(val)
			err = f.Push(varr)
			if err != nil {
				return NIL, NewExecutionError("STV push var failed").Wrap(err)
			}
			f.ip++

		case OPLDV:
			// note this avoids pop-push dance
			idx := f.sp - 1
			if idx < 0 {
				return NIL, NewExecutionError("LDV stack underflow")
			}
			varr, ok := f.stack[idx].(*Var)
			if !ok {
				return NIL, NewExecutionError("LDV invalid var on stack")
			}
			f.stack[idx] = varr.Deref()
			f.ip++

		case OPLDK:
			idx, err := f.code.Get32(f.ip + 1)
			if err != nil {
				return NIL, NewExecutionError("get closed over index failed").Wrap(err)
			}
			// FIXME cache closedOvers count
			if idx >= len(f.closedOvers) {
				return NIL, NewExecutionError("closed over lookup out of bounds")
			}
			err = f.Push(f.closedOvers[idx])
			if err != nil {
				return NIL, NewExecutionError("closed over push failed").Wrap(err)
			}
			f.ip += 5

		case OPPAK:
			val, err := f.Pop()
			if err != nil {
				return NIL, NewExecutionError("popping closed over value failed").Wrap(err)
			}
			idx := f.sp - 1
			if idx < 0 {
				return NIL, NewExecutionError("PAK stack overflow").Wrap(err)
			}
			cls := f.stack[idx]
			if cls.Type() != FuncType {
				return NIL, NewExecutionError("PAK expected a Fn")
			}
			fun := cls.(*Func)
			fun.closedOvers = append(fun.closedOvers, val)
			f.ip++

		default:
			return NIL, NewExecutionError("unknown instruction")
		}

	}
}
