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

package main

import (
	"bufio"
	"fmt"
	"github.com/nooga/let-go/pkg/compiler"
	"github.com/nooga/let-go/pkg/vm"
	"log"
	"os"
)

func makeNamespace() (*vm.Namespace, error) {
	// FIXME make a stdlib that will declare these
	plus, err := vm.NativeFnType.Box(func(a int, b int) int { return a + b })
	mul, err := vm.NativeFnType.Box(func(a int, b int) int { return a * b })
	sub, err := vm.NativeFnType.Box(func(a int, b int) int { return a - b })
	printlnf, err := vm.NativeFnType.Box(fmt.Println)
	if err != nil {
		return nil, err
	}

	ns := vm.NewNamespace("user")
	ns.Def("+", plus)
	ns.Def("*", mul)
	ns.Def("-", sub)
	ns.Def("println", printlnf)
	return ns, nil
}

func main() {
	ns, err := makeNamespace()
	if err != nil {
		fmt.Println("init error:", err)
		return
	}
	comp := compiler.NewCompiler(ns)

	scanner := bufio.NewScanner(os.Stdin)
	prompt := "=> "
	fmt.Print(prompt)
	for scanner.Scan() {
		in := scanner.Text()
		chunk, err := comp.Compile(in)
		val, err := vm.NewFrame(chunk, nil).Run()
		if err != nil {
			fmt.Println(err)
		}

		fmt.Println(val.Unbox())
		fmt.Print(prompt)
	}

	if err := scanner.Err(); err != nil {
		log.Println(err)
	}
}