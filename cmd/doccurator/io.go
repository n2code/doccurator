package main

import (
	"bufio"
	"fmt"
	"github.com/n2code/doccurator"
	"golang.org/x/term"
	"os"
	"os/signal"
	"strings"
	"unicode"
)

func PromptUser(allowEscapeSequences bool) doccurator.RequestChoice {
	return func(request string, options []string, cleanup bool) (choice string) {
		letterToChoice := make(map[rune]string)
		var displayOptions []string

	ParseOptions:
		for _, option := range options {
			for i, letter := range option {
				if _, taken := letterToChoice[letter]; !taken {
					lowerLetter := unicode.ToUpper(letter)
					upperLetter := unicode.ToLower(letter)
					letterToChoice[lowerLetter] = option
					letterToChoice[upperLetter] = option
					printLetter := fmt.Sprintf("\x1B[1m\x1B[4m%c\x1B[0m", letter)
					if !allowEscapeSequences {
						printLetter = fmt.Sprintf("[%c]", letter)
					}
					displayOptions = append(displayOptions, fmt.Sprintf("%s%s%s", option[:i], printLetter, option[i+1:]))
					continue ParseOptions
				}
			}
		}

		key := make(chan rune)
		interrupt := make(chan os.Signal, 1)

		signal.Notify(interrupt, os.Interrupt)
		defer func() { signal.Reset(os.Interrupt) }()

		rawMode := false
		out := func(text string) {
			fmt.Fprint(os.Stdout, text)
		}
		rawOut := func(text string) {
			if rawMode {
				fmt.Fprint(os.Stdout, text)
			}
		}

		if allowEscapeSequences {
			if oldTermState, err := term.MakeRaw(int(os.Stdin.Fd())); err == nil {
				rawMode = true
				defer term.Restore(int(os.Stdin.Fd()), oldTermState)
			} // else terminal is not raw, i.e. ENTER is required to confirm input -> acceptable fallback
		}
		waitForKey := func() {
			reader := bufio.NewReaderSize(os.Stdin, 1)
			input, _ := reader.ReadByte()
			if !rawMode && reader.Buffered() > 0 {
				if extra, _ := reader.ReadByte(); extra != '\n' && extra != '\r' {
					key <- '?'
					reader.Reset(os.Stdin)
					return
				}
			}
			reader.Reset(os.Stdin)
			if rawMode && input == 3 { //Ctrl+C
				interrupt <- os.Interrupt
			} else {
				rawOut(fmt.Sprintf("%c", unicode.ToUpper(rune(input))))
				key <- rune(input)
			}
		}

		prompt := fmt.Sprintf("%s (%s): ", request, strings.Join(displayOptions, " / "))
		out(prompt)
		for {
			go waitForKey()
			select {
			case letterPressed := <-key:
				if selection, found := letterToChoice[letterPressed]; found {
					if cleanup {
						rawOut("\033[2K\r") //clear line
					} else {
						rawOut("\r\n")
					}
					return selection
				}
				rawOut("\a\033[1D") //bell and move cursor left by 1
				if !rawMode {
					out(prompt)
				}
			case <-interrupt:
				out("<CANCELLED>\r\n")
				return "" //represents abort as per type documentation
			}
		}
	}
}

func AutoChooseDefaultOption(quiet bool) doccurator.RequestChoice {
	return func(request string, options []string, cleanup bool) string {
		defaultChoice := options[0] //by definition of type RequestChoice
		if !cleanup && !quiet {
			fmt.Fprintf(os.Stdout, "%s => [%s]\n", request, strings.ToUpper(defaultChoice))
		}
		return defaultChoice
	}
}
