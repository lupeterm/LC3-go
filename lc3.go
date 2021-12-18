package main

import (
	"fmt"
	"os"
)

var mem [65535]uint16
var reg [R_COUNT]uint16
var keyBuffer []rune

/*   registers of LC-3 */
const (
	R_R0    = iota
	R_R1    = iota
	R_R2    = iota
	R_R3    = iota
	R_R4    = iota
	R_R5    = iota
	R_R6    = iota
	R_R7    = iota
	R_PC    = iota
	R_COND  = iota
	R_COUNT = iota
)

/*  opcodes  */
const (
	OP_BR   = iota
	OP_ADD  = iota
	OP_LD   = iota
	OP_ST   = iota
	OP_JSR  = iota
	OP_AND  = iota
	OP_LDR  = iota
	OP_STR  = iota
	OP_RTI  = iota
	OP_NOT  = iota
	OP_LDI  = iota
	OP_STI  = iota
	OP_JMP  = iota
	OP_RES  = iota
	OP_LEA  = iota
	OP_TRAP = iota
)

/* sign of previous operation */
const (
	FL_POS = 1 << 0
	FL_ZRO = 1 << 1
	FL_NEG = 1 << 2
)

const (
	TRAP_GETC  = 0x20 // get char from keyboard not echoed by terminal
	TRAP_OUT   = 0x21 // output a char
	TRAP_PUTS  = 0x22 // output a word string
	TRAP_IN    = 0x23 // get character from keyboard and echo
	TRAP_PUTSP = 0x24 // output a byte string
	TRAP_HALT  = 0x25 // halt the program
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("LC3 [image-file1]..")
		os.Exit(2)
	}

	// for _, arg := range os.Args {
	//  read image
	// }
	const (
		PC_START = 0x3000 // leave space for the trap routine code
	)
	reg[R_PC] = PC_START

	running := 1
	for running != 0 {
		var instr, op uint16
		instr = memRead(reg[R_PC] + 1)
		op = instr >> 12
		switch op {
		case OP_ADD:
			/*
				16 bits:
				ADD R0 R1 R2;  :
				15-12 0001 => 1(ADD)
				11- 9 destination register (r0)
				8 - 6 left operand (r1)
				5     immediate mode / register mode
				      immediate mode: second value is stored inside the instruction itself and gets added onto r1 and stored into r0
					  register mode: second value is in another register and gets added onto r1 and stored into r0
				4 - 3 unused
				2 - 0 right operand (r2)

			*/
			r0 := (instr >> 9) & 0x7
			r1 := (instr >> 6) & 0x7
			imm5 := (instr >> 5) & 0x1

			if imm5 == 1 {
				imm5 := signExtend(instr&0x1F, 5)
				reg[r0] = reg[r1] + imm5
			} else {
				r2 := instr & 0x7
				reg[r0] = reg[r1] + reg[r2]
			}
			updateFlags(r0)

		case OP_AND:
			/*
				16 bits:
				AND DR SR! SR2;  :
				15-12 0101 => 5 (AND)
				11- 9 destination register (r0)
				8 - 6 left operand (r1)
				5     immediate mode / register mode
						immediate mode: second value is stored inside the instruction itself and gets added onto r1 and stored into r0
						register mode: second value is in another register and gets added onto r1 and stored into r0
				4 - 3 unused
				2 - 0 right operand (r2)
			*/
			r0 := (instr >> 9) & 0x7
			r1 := (instr >> 6) & 0x7
			imm5 := (instr >> 5) & 0x1

			if imm5 == 1 {
				imm5 = signExtend(instr&0x1F, 5) // sign extending to 16 bits
				reg[r0] = reg[r1] & imm5
			} else {
				r2 := instr & 0x7
				reg[r0] = reg[r1] & reg[r2]
			}
			updateFlags(r0)
		case OP_NOT:
			/*
				16 bits:
				15-12 1001 => 9 (NOT)
				11-9 DR r0
				8-6  SR r1
				5    1
				4-0  11111
			*/
			r0 := (instr >> 9) & 0x7
			r1 := (instr >> 6) & 0x7
			reg[r0] = ^reg[r1]
			updateFlags(r0)
		case OP_BR:
			/*
				16 bits:
				15-12 0000 => 0 (BRANCH)
				11  negative
				10  zero
				9   positive
				8 - 0 PCOffset9
			*/
			PCOffset := signExtend(instr&0x1FF, 9)
			condFlag := (instr >> 9) & 0x7
			if condFlag&reg[R_COND] != 0 {
				reg[R_PC] += PCOffset
			}

		case OP_JMP:
			/*
				16 bits:
				15-12 1100 => 12 (JUMP)
				11- 9 unused
				8 - 6 base register ( the register that will be jumped to)
				5 - 0 unused
			*/
			r1 := (instr >> 6) & 0x7
			reg[R_PC] = reg[r1]
		case OP_JSR:
			/*
					JSR => Jump to Subroutine
					16 bits:
					15-12 0100 => 4 opcode
					11    if addr of subroutine is obtained by base register,
						  otherwise it is obtained by sign extending an offset
				    	  10-0 PCOffset11  !!
					10-9  unused
					8 - 6 base register
					5- 0 unused
			*/
			reg[R_R7] = reg[R_PC]
			offsetFlag := (instr >> 11) & 1
			if offsetFlag == 0 {
				r1 := (instr >> 6) & 0x7
				reg[R_PC] = reg[r1]
			} else {
				PCOffset := signExtend(instr&0x7FF, 11)
				reg[R_PC] += PCOffset
			}

		case OP_LD:
			/*  16 bits:
			15-12: opcode 0010 => 2 (Load)
			11- 9: destination register (dr)
			8 - 0: PCoffset9  LD is Limited to 9 bits, LDI is useful for loading from addresses far away
			*/
			r0 := (instr >> 9) & 0x7
			PCOffset9 := signExtend(instr&0x1FF, 9)
			reg[r0] = memRead(reg[R_PC] + PCOffset9)
			updateFlags(r0)
		case OP_LDI:
			/*  16 bits:
			15-12: opcode 1010 => 19 (Load Indirect)
			11- 9: destination register (dr)
			8 - 0: PCoffset9  LD is Limited to 9 bits, LDI is useful for loading from addresses far away
			*/
			r0 := (instr >> 9) & 0x7
			PCOffset9 := signExtend(instr&0x1FF, 9)
			reg[r0] = memRead(memRead(reg[R_PC] + PCOffset9))
			updateFlags(r0)
		case OP_LDR:
			/*  16 bits:
			15-12: opcode 0110 => 6 (Load base + offset)
			11- 9: destination register (dr)
			8-  6: Base register baseR
			5 - 0: offset6
			*/
			r0 := (instr >> 9) & 0x7
			r1 := (instr >> 6) & 0x7
			offset := signExtend(instr&0x3F, 6)
			reg[r0] = memRead(r1 + offset)
			updateFlags(r0)
		case OP_LEA:
			/*  16 bits:
			15-12: opcode 1110 => 14 (Load effective Areas)
			11- 9: destination register (dr)
			8 - 0: PCoffset9
			*/
			r0 := (instr >> 9) & 0x7
			PCoffset9 := signExtend(instr&0x1FF, 9)
			reg[r0] = memRead(reg[R_PC] + PCoffset9)
			updateFlags(r0)
		case OP_ST:
			/*  16 bits:
			15-12: opcode 0011 => 3 (Store)
			11- 9  SR (r0)
			8 - 0: PCoffset9
			*/
			r0 := (instr >> 9) & 0x7
			PCOffset9 := signExtend(instr&0x1FF, 9)
			memWrite(reg[R_PC]+PCOffset9, reg[r0])
		case OP_STI:
			/*  16 bits:
			15-12: opcode 1011 => 11 (Store indirect)
			11- 9  SR (r0)
			8 - 0: PCoffset9
			*/
			r0 := (instr >> 9) & 0x7
			PCOffset9 := signExtend(instr&0x1FF, 9)
			memWrite(memRead(reg[R_PC]+PCOffset9), reg[r0])
		case OP_STR:
			/*  16 bits:
			15-12: opcode 0111 => 7 (Store base + offset)
			11- 9  SR (r0)
			8 - 6 baseR (r1)
			5 - 0: PCoffset6
			*/
			r0 := (instr >> 9) & 0x7
			r1 := (instr >> 6) & 0x7
			PCOffset6 := signExtend(instr&0x3F, 6)
			memWrite(reg[r1]+PCOffset6, reg[r0])
		case OP_TRAP:
			/*
				16 bits:
				15-12 opcode 1111 => 15 (TRAP)
				11-8  0000
				7- 0  trapvect8
			*/
			switch instr & 0xFF {
			case TRAP_GETC:
				for {
					if len(keyBuffer) > 0 {
						break
					}
				}
				reg[R_R0], keyBuffer = uint16(keyBuffer[0]), keyBuffer[1:]
			case TRAP_OUT:
				chr := rune(reg[R_R0])
				fmt.Printf("%c", chr)
			case TRAP_PUTS:
				r0 := reg[R_R0]
				var chr, i uint16
				for ok := true; ok; ok = (chr != 0x0) {
					chr = mem[r0+i] & 0xFFFF
					fmt.Printf("%c", rune(chr))
					i++
				}
			default:
				fmt.Printf("Trap code not implemented: 0x%04X\n", instr)
			}
		case OP_RES:
			fmt.Println("NOT IMPLEMENTED YET!")
			os.Exit(2)
		case OP_RTI:
			fmt.Println("NOT IMPLEMENTED YET!")
			os.Exit(2)
		default:
			fmt.Println("Bad OPCODE! ")
			os.Exit(2)
		}
		running = 0
	}
}

const (
	MR_KBSR = 0xFE00
	MR_KBDR = 0xFE02
)

func memRead(address uint16) uint16 {
	if address == MR_KBSR {
		memWrite(MR_KBSR, memRead(MR_KBSR)&0x7FFF)
	}
	if address <= 65535 {
		return uint16(mem[address])
	} else {
		return 0
	}
}

func memWrite(address uint16, val uint16) {
	mem[address] = val
}
func updateFlags(rIndex uint16) {
	if reg[rIndex] == 0 {
		reg[R_COND] = FL_ZRO
	} else if reg[rIndex]>>15 == 1 {
		reg[R_COND] = FL_NEG
	} else {
		reg[R_COND] = FL_POS
	}
}

func signExtend(x uint16, bitCount int) uint16 {
	if (x >> (bitCount - 1) & 1) != 0 {
		x |= (0xFFFF << bitCount)
	}
	return x
}
