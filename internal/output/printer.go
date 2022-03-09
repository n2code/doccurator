package output

import (
	"fmt"
	"io"
	"os"
)

type Class int

const (
	Required Class = iota
	Error
	Normal
	Verbose
)

type Printer struct {
	classes    map[Class]bool
	terminal   io.Writer
	diagnosis  io.Writer
	useEscapes bool
}

func NewPrinter(include []Class, allowEscapes bool) (p Printer) {
	p = Printer{
		classes:    map[Class]bool{},
		terminal:   os.Stdout,
		diagnosis:  os.Stderr,
		useEscapes: allowEscapes,
	}
	for _, class := range include {
		p.classes[class] = true
	}
	return
}

func (p Printer) Out(class Class, format string, values ...interface{}) {
	if !p.classes[class] {
		return
	}
	target := &p.terminal
	if class == Error {
		target = &p.diagnosis
	}
	fmt.Fprintf(*target, format, values...)
}
