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

func (p Printer) ClassifiedPrintf(class Class, format string, values ...interface{}) {
	if !p.classes[class] {
		return
	}

	target := &p.terminal
	if class == Error {
		target = &p.diagnosis
	}

	fmt.Fprintf(*target, format, p.adjustModifiers(values...)...)
}

func (p Printer) Sprintf(format string, values ...interface{}) string {
	return fmt.Sprintf(format, p.adjustModifiers(values...)...)
}

func (p Printer) adjustModifiers(values ...interface{}) (replacements []interface{}) {
	for _, value := range values {
		if _, isModifier := value.(SgrModifier); isModifier && !p.useEscapes {
			replacements = append(replacements, "") //append empty string to satisfy fmt verbs
			continue
		}
		replacements = append(replacements, value)
	}
	return
}
