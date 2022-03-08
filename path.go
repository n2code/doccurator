package doccurator

import (
	"fmt"
	"github.com/n2code/doccurator/internal"
	"github.com/n2code/doccurator/internal/document"
	"github.com/n2code/doccurator/internal/output"
	"github.com/n2code/ndocid"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

//represents file.23456X777.ndoc.ext or file_without_ext.23456X777.ndoc or .23456X777.ndoc.ext_only
var ndocFileNameRegex = regexp.MustCompile(`^.*\.(` + document.IdPattern + `)\.ndoc(?:\.[^.]*)?$`)

// ExtractIdFromStandardizedFilename attempts to discover and extract a valid ID from the given filename or path.
func ExtractIdFromStandardizedFilename(path string) (document.Id, error) {
	filename := filepath.Base(path)
	matches := ndocFileNameRegex.FindStringSubmatch(filename)
	if matches == nil {
		return 0, fmt.Errorf("ID missing in filename %s (expected format <name>.<ID>.ndoc.<ext>, e.g. notes.txt.23352M4R96Z.ndoc.txt)", filename)
	}
	textId := matches[1]
	numId, err, _ := ndocid.Decode(textId)
	if err != nil {
		return 0, fmt.Errorf(`bad ID in filename %s (%w)`, filename, err)
	}
	return document.Id(numId), nil
}

const libRootScheme = "lib:" + string(filepath.Separator) + string(filepath.Separator)

func (d *doccurator) displayablePath(absolutePath string, shortenLibraryRoot bool, omitDotSlash bool) string {
	pleasant := pleasantPath(filepath.Clean(absolutePath), d.appLib.GetRoot(), mustGetwd(), shortenLibraryRoot, omitDotSlash)
	if d.fancyTerminalFeatures && strings.HasPrefix(pleasant, libRootScheme) {
		pleasant = strings.Replace(pleasant, libRootScheme, output.TerminalFormatAsDim(libRootScheme), 1)
	}
	return pleasant
}

const dot string = "."
const dirSeparator = string(filepath.Separator)
const dotDirSeparator = dot + dirSeparator
const doubleDot = dot + dot
const doubleDotDirSeparator = doubleDot + dirSeparator

func isChildOf(child string, parent string) bool {
	rel, err := filepath.Rel(parent, child)
	internal.AssertNoError(err, "paths should both be nice and not of mixed nature")
	return !(rel == dot || rel == doubleDot || strings.HasPrefix(rel, doubleDotDirSeparator))
}

// pleasantPath turns an absolute path into something easily understandable from the current context.
// If the working directory is inside the library a relative path is emitted, with leading "./" to stress relativity (opt-out possible).
// If the current location is outside the library an anchored path is printed and the library root is abbreviated.
// If the [absolute] input path is a target outside the library it is reflected unchanged.
func pleasantPath(absolute string, root string, wd string, collapseRoot bool, omitDotSlash bool) string {
	if wdAboveRoot := isChildOf(root, wd); wdAboveRoot {
		if !collapseRoot {
			return absolute
		}
		anchored, _ := filepath.Rel(root, absolute) //error impossible because both are rooted
		return libRootScheme + anchored
	}

	prefix := ""
	relative, _ := filepath.Rel(wd, absolute) //error impossible because both are rooted
	if !omitDotSlash && !strings.HasPrefix(relative, doubleDotDirSeparator) {
		prefix = dotDirSeparator
	}
	return prefix + relative
}

func mustGetwd() string {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return wd
}

// mustAbsFilepath calls filepath.Abs and asserts that it is successful
func mustAbsFilepath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		panic(err)
	}
	return abs
}

// mustRelFilepathToWorkingDir calculates the path relative to the given target path (using the current working directory) and asserts that it is successful
func mustRelFilepathToWorkingDir(target string) string {
	rel, err := filepath.Rel(mustGetwd(), target)
	if err != nil {
		panic(err)
	}
	return rel
}
