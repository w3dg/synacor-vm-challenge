package vm

import (
	"bufio"
	"fmt"
	"log"
	"log/slog"
	"os"
)

const (
	UINT16_MAX_VALUE = 1<<16 - 1 // Maximum value that can be represented by 16 bits
	UINT16_MAX_LEN   = 1 << 16   // Total number of values that can be represented by 16 bits

	UINT15_MAX_VALUE = 1<<15 - 1 // Maximum value that can be represented by 15 bits
	UINT15_MAX_LEN   = 1 << 15   // Total number of values that can be represented by 15 bits

	VAL_REGIDX = "REGISTER" // The value resolved was to be a register
	VAL_LIT    = "LITERAL"  // The value resolved refers to a literal value
)

type Value uint16

type VM struct {
	isHalted    bool                  // Is the vm halted
	memory      [UINT16_MAX_LEN]Value // 15 bit address space with 16 bit values (Value)
	registers   [8]Value              // 8 Registers with 16 bit values (Value)
	inputBuffer []byte                // Input Buffer for user input, read character from user
	inputBufPos int                   // Index of how far the input buffer we've consumed, as program
	// takes value one char at a time but we can buffer it safely
	stack []Value // Unbounded stack to store 16 bit values (Value)
	sp    int     // Stack pointer
	ip    int     // Instruction pointer, points to the start of the current instruction in VM.memory
}

type ResolvedValue struct {
	// 16bit value, either refers to a literal value or a register index, depending on the type
	rVal Value
	// Indicates whether the resolved value was to refer to a register index or a literal value
	rType string
}

// resolveValue resolves the values passed in as input as per the following rules:
// Numbers 0..32767 mean a literal value
// Numbers 32768..32775 instead mean registers 0..7
// Numbers 32776..65535 are invalid
func (vm *VM) resolveValue(v Value) ResolvedValue {
	if v <= UINT15_MAX_VALUE { // literal values, 0..32767
		return ResolvedValue{v, VAL_LIT}
	} else if v >= UINT15_MAX_LEN && v < UINT15_MAX_LEN+8 { // 8 registers, 32768..32775
		idx := v - UINT15_MAX_LEN
		return ResolvedValue{vm.registers[idx], VAL_REGIDX}
	} else {
		panic(fmt.Sprintf("Invalid value passed to resolve %d", v))
	}
}

func (vm *VM) SaveValue(destaddr, val Value) {
	if destaddr <= UINT15_MAX_VALUE { // 15 bit address space memory addresses, 0..32767
		vm.memory[destaddr] = val
	} else if destaddr >= UINT15_MAX_LEN && destaddr < UINT15_MAX_LEN+8 { // 8 registers, 32768..32775
		idx := destaddr - UINT15_MAX_LEN
		vm.registers[idx] = val
	} else {
		panic(fmt.Sprintf("Invalid Destination, neither mem or registers passed %d", destaddr))
	}
}

func New() *VM {
	return &VM{
		isHalted: false, // let it start
		sp:       -1,    // set stack pointer to -1, empty stack
		ip:       0,     // set inst pointer to 0
	}
}

func (vm *VM) SetMemory(idx Value, value []byte) {
	vm.memory[idx] = BytesToValue(value)
}

func (vm *VM) DumpState() {
	if vm.isHalted {
		slog.Debug(fmt.Sprint("Halted", vm.isHalted))
	}
	slog.Debug(fmt.Sprint("Stack Pointer", vm.sp))
	slog.Debug(fmt.Sprint("Instruction Pointer", vm.ip))
	slog.Debug(fmt.Sprint("Registers", vm.registers))
}

func (vm *VM) DumpMem(idx Value) {
	slog.Debug("Memory Locations")
	for i, v := range vm.memory[idx-5 : idx+5] {
		slog.Debug(fmt.Sprint("index: ", int(idx)-5+i, "val: ", v))
	}
}

func (vm *VM) Halt() {
	if vm.isHalted {
		panic("VM is already halted. Received another halt instruction")
	}
	vm.isHalted = true
}

// an unbounded stack which holds individual 16-bit values
func (vm *VM) Push(v Value) {
	if vm.sp == -1 {
		vm.stack = []Value{v}
		vm.sp = 0
		return
	}

	if vm.sp+1 == len(vm.stack) {
		vm.stack = append(vm.stack, v)
	} else {
		vm.stack[vm.sp+1] = v
	}
	vm.sp++
}

// remove the top element from the stack return it; empty stack = error
func (vm *VM) Pop() Value {
	if vm.sp == -1 {
		panic("Stack underflow")
	}
	val := vm.stack[vm.sp]
	vm.sp--
	return val
}

// Takes a byte array of length 2 (or panics otherwise).
// Assumes the 2 byte array to be in little endian.
// Converts the values in little endian to a (big endian) 16 bit value.
func BytesToValue(b []byte) Value {
	if len(b) != 2 {
		panic("byte array to convert does not add up to 16 bits")
	}

	low, high := b[0], b[1]
	val := (Value(high) << 8) | Value(low)

	return val
}

// // Takes a value to be converted from Value to a value to be
// // stored as little endian back in memory.
// // A two byte array is returned as values are 16 bits, i.e. 2 bytes
// func ValueToBytes(val Value) []byte {
// 	highMask := 0xff00
// 	lowMask := 0x00ff

// 	highByte := byte(val & Value(highMask) >> 8)
// 	lowByte := byte(val & Value(lowMask))

// 	return []byte{lowByte, highByte}
// }

func (vm *VM) Step() bool {
	if vm.isHalted {
		panic("Tried to start vm while halted")
	}

	opcode := vm.memory[vm.ip]

	switch opcode {
	case 0:
		//halt: 0
		//   stop execution and terminate the program
		slog.Info("halt")
		vm.isHalted = true
		vm.ip += 1
		return false

	case 1:
		// set: 1 a b
		//   set register <a> to the value of <b>
		op1 := vm.memory[vm.ip+1]
		op2 := vm.memory[vm.ip+2]

		regIdx := op1 % UINT15_MAX_LEN
		val := vm.resolveValue(op2)

		slog.Debug(fmt.Sprint("set", opcode, op1, op2))
		vm.registers[regIdx] = val.rVal
		vm.ip += 3

	case 2:
		// push: 2 a
		//   push <a> onto the stack
		op1 := vm.memory[vm.ip+1]

		slog.Debug(fmt.Sprint("push", opcode, op1))
		a := op1
		if a > UINT15_MAX_VALUE {
			a = vm.registers[a%UINT15_MAX_LEN]
		}
		vm.Push(a)
		vm.ip += 2

	case 3:
		// pop: 3 a
		//   remove the top element from the stack and write it into <a>; empty stack = error
		op1 := vm.memory[vm.ip+1]

		slog.Debug(fmt.Sprint("pop", opcode, op1))

		vm.registers[op1%UINT15_MAX_LEN] = vm.Pop()

		vm.ip += 2

	case 4, 5:
		// eq: 4 a b c
		//   set <a> to 1 if <b> is equal to <c>; set it to 0 otherwise
		// gt: 5 a b c
		//   set <a> to 1 if <b> is greater than <c>; set it to 0 otherwise
		op1 := vm.memory[vm.ip+1]
		op2 := vm.memory[vm.ip+2]
		op3 := vm.memory[vm.ip+3]

		slog.Debug("start op")

		b := vm.resolveValue(op2).rVal
		c := vm.resolveValue(op3).rVal

		slog.Debug(fmt.Sprint("raw eq(4) or gt(5)", opcode, op1, op2, op3))
		slog.Debug(fmt.Sprint("eq(4) or gt(5)", opcode, op1, b, c))
		switch opcode {

		case 4:
			if b == c {
				vm.SaveValue(op1, 1)
			} else {
				vm.SaveValue(op1, 0)
			}
		case 5:
			if b > c {
				vm.SaveValue(op1, 1)
			} else {
				vm.SaveValue(op1, 0)
			}
		}
		vm.ip += 4

	case 6:
		// jmp: 6 a
		//   jump to <a>
		op1 := vm.memory[vm.ip+1]
		targetVal := vm.resolveValue(op1)
		target := targetVal.rVal

		slog.Debug(fmt.Sprint("jmp", opcode, op1))

		// if targetVal.rType == VAL_REGIDX {
		// 	target = vm.registers[target]
		// }
		vm.ip = int(target)

	case 7:
		// jt: 7 a b
		//   if <a> is nonzero, jump to <b>
		op1 := vm.memory[vm.ip+1]
		op2 := vm.memory[vm.ip+2]

		op1 = vm.resolveValue(op1).rVal
		op2 = vm.resolveValue(op2).rVal

		slog.Debug(fmt.Sprint("jt", opcode, op1, op2))

		if op1 != 0 {
			vm.ip = int(op2)
			slog.Debug("jumped")
		} else {
			vm.ip += 3
		}

	case 8:
		// jf: 8 a b
		//   if <a> is zero, jump to <b>
		op1 := vm.memory[vm.ip+1]
		op2 := vm.memory[vm.ip+2]
		slog.Debug(fmt.Sprint("jf", opcode, op1, op2))

		op1 = vm.resolveValue(op1).rVal
		op2 = vm.resolveValue(op2).rVal

		if op1 == 0 {
			vm.ip = int(op2)
			slog.Debug("jumped")
		} else {
			vm.ip += 3
		}

	case 9, 10, 11:
		// add: 9 a b c
		//   assign into <a> the sum of <b> and <c> (modulo 32768)
		// mult: 10 a b c
		//   store into <a> the product of <b> and <c> (modulo 32768)
		// mod: 11 a b c
		//   store into <a> the remainder of <b> divided by <c>
		op1 := vm.memory[vm.ip+1]
		op2 := vm.resolveValue(vm.memory[vm.ip+2]).rVal
		op3 := vm.resolveValue(vm.memory[vm.ip+3]).rVal

		slog.Debug(fmt.Sprint("add(9), mult(10), mod(11)", opcode, op1, op2, op3))

		var result int
		switch opcode {
		case 9: // a = b + c
			result = (int(op2) + int(op3)) % UINT15_MAX_LEN
		case 10: // a = b * c
			result = (int(op2) * int(op3)) % UINT15_MAX_LEN
		case 11: // a = b % c
			result = (int(op2) % int(op3))
		}
		vm.SaveValue(op1, Value(result))
		vm.ip += 4

	case 12, 13:
		// and: 12 a b c
		//   stores into <a> the bitwise and of <b> and <c>
		// or: 13 a b c
		//   stores into <a> the bitwise or of <b> and <c>

		op1 := vm.memory[vm.ip+1]
		op2 := vm.resolveValue(vm.memory[vm.ip+2]).rVal
		op3 := vm.resolveValue(vm.memory[vm.ip+3]).rVal

		slog.Debug(fmt.Sprint("and(12), or(13)", opcode, op1, op2, op3))
		var res Value
		switch opcode {
		case 12: // and
			res = op2 & op3
		case 13: // or
			res = op2 | op3
		}
		vm.SaveValue(op1, res)

		vm.ip += 4

	case 14:
		// not: 14 a b
		//   stores 15-bit bitwise inverse of <b> in <a>
		op1 := vm.memory[vm.ip+1]
		op2 := vm.memory[vm.ip+2]

		slog.Debug(fmt.Sprint("not", opcode, op1, op2))
		b := vm.resolveValue(op2).rVal
		res := (^b) & Value(UINT15_MAX_VALUE) // flip all bits, mask to 15 bits
		vm.SaveValue(op1, Value(res))

		vm.ip += 3

	case 15:
		// rmem: 15 a b
		//   read memory at address <b> and write it to <a>
		op1 := vm.memory[vm.ip+1]
		op2 := vm.memory[vm.ip+2]

		slog.Debug(fmt.Sprint("rmem", opcode, op1, op2))

		// read "mem" at address b (memory or registers), hence resolve b first
		addr := vm.resolveValue(op2).rVal
		target := vm.memory[addr]

		// write it at <a>
		vm.SaveValue(op1, target)
		vm.ip += 3

	case 16:
		// wmem: 16 a b
		//   write the value from <b> into memory at address <a>
		op1 := vm.memory[vm.ip+1]
		op2 := vm.memory[vm.ip+2]

		// a can be a register holding an address, resolve it
		addr := vm.resolveValue(op1).rVal
		val := vm.resolveValue(op2).rVal

		vm.memory[addr] = val
		vm.ip += 3

	case 17:
		// call: 17 a
		//   write the address of the next instruction to the stack and jump to <a>
		op1 := vm.memory[vm.ip+1]

		slog.Debug(fmt.Sprint("call", opcode, op1))

		// save next inst
		saveIP := vm.ip + 2
		vm.Push(Value(saveIP))

		// jump
		op1 = vm.resolveValue(op1).rVal
		vm.ip = int(op1)
	case 18:
		// ret: 18
		//   remove the top element from the stack and jump to it; empty stack = halt
		slog.Debug(fmt.Sprint("ret", opcode))
		if len(vm.stack) == 0 {
			slog.Info("Empty stack return, halting")
			vm.Halt()
		}
		retIP := vm.Pop()
		vm.ip = int(retIP)

	case 19:
		// out: 19 a
		//   write the character represented by ascii code <a> to the terminal
		op1 := vm.resolveValue(vm.memory[vm.ip+1]).rVal
		fmt.Printf("%c", op1)
		vm.ip += 2

	case 20:
		// in: 20 a
		// 	 read a character from the terminal and write its ascii code to <a>; it
		//   can be assumed that once input starts, it will continue until a newline is
		// 	  encountered; this means that you can safely read whole lines from the
		// 	  keyboard instead   of having to figure out how to read individual characters

		target := vm.memory[vm.ip+1]

		if vm.inputBufPos == len(vm.inputBuffer) {
			reader := bufio.NewReader(os.Stdin)
			line, err := reader.ReadString('\n') // read upto the delimiter \n
			if err != nil {
				log.Fatal("Could not read line upto newline")
			}
			vm.inputBuffer = []byte(line)
			vm.inputBufPos = 0
		}

		// write one byte at a time regardless of buffer resets
		character := vm.inputBuffer[vm.inputBufPos]
		vm.SaveValue(target, Value(character))
		vm.inputBufPos++

		vm.ip += 2

	case 21:
		// noop: 21
		//   no operation
		vm.ip += 1

	default:
		log.Fatalf("Unknown instruction %v", opcode)
	}

	slog.Debug(fmt.Sprintf("New ip: %v", vm.ip))
	return true
}
