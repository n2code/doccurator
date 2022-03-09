package doccurator

import (
	"fmt"
	"github.com/n2code/doccurator/internal"
	"github.com/n2code/doccurator/internal/document"
	"github.com/n2code/doccurator/internal/library"
	out "github.com/n2code/doccurator/internal/output"
	"path/filepath"
	"strings"
)

func (d *doccurator) PrintRecord(id document.Id) {
	doc, exists := d.appLib.GetDocumentById(id)
	if exists {
		d.Print(out.Required, "%s\n", doc)
	}
}

func (d *doccurator) PrintAllRecords(excludeRetired bool) {
	d.Print(out.Normal, "Library: %s\n\n\n", d.appLib.GetRoot())
	count := 0
	d.appLib.VisitAllRecords(func(doc library.Document) {
		if doc.IsObsolete() && excludeRetired {
			return
		}
		d.Print(out.Required, "%s\n\n", doc)
		count++
	})
	if count == 0 {
		d.Print(out.Normal, "<no records>")
	} else {
		d.Print(out.Normal, "\n%d in total\n", count)
	}
}

func (d *doccurator) PrintTree(excludeUnchanged bool, onlyWorkingDir bool) error {
	displayFilters := make([]library.PathSkipEvaluator, 0, 2)
	libRoot := d.appLib.GetRoot()

	label := libRoot + " [library root]"
	trimPrefix := ""

	if onlyWorkingDir {
		wd := mustGetwd()
		if isChildOf(wd, libRoot) {
			label = wd + " [working directory]"

			trimPrefix, _ = filepath.Rel(libRoot, wd)
			trimPrefix += dirSeparator

			displayFilters = append(displayFilters, func(absolute string, dir bool) bool {
				//do not skip if...
				return !(dir && isChildOf(wd, absolute) || //directory above (walk-into required!)
					dir && absolute == wd || //or working directory itself
					isChildOf(absolute, wd)) //or file/directory inside working directory
			})
		} else if wd != libRoot {
			return fmt.Errorf("working directory is not inside library")
		} //else wd == root which shall not behave differently
	}

	tree := out.NewVisualFileTree(label)

	var pathsWithErrors []*library.CheckedPath
	paths, ok := d.appLib.Scan([]library.PathSkipEvaluator{d.isLibFile}, displayFilters, d.optimizedFsAccess) //full scan may optimize performance if allowed to
	for index, checkedPath := range paths {
		prefix := ""
		status := checkedPath.Status()
		if excludeUnchanged && !status.RepresentsChange() {
			continue
		}
		if status != library.Tracked {
			prefix = fmt.Sprintf("[%c] ", status)
		}
		tree.InsertPath(strings.TrimPrefix(checkedPath.AnchoredPath(), trimPrefix), prefix)
		if status == library.Error {
			pathsWithErrors = append(pathsWithErrors, &paths[index])
		}
	}
	errorCount := len(pathsWithErrors)

	d.Print(out.Required, "%s", tree.Render())

	//TODO [FEATURE]: coloring
	if !ok {
		var msg strings.Builder
		fmt.Fprintf(&msg, "%d scanning %s occurred:", errorCount, out.Plural(errorCount, "error", "errors"))
		for _, errorPath := range pathsWithErrors {
			fmt.Fprintf(&msg, "\n@%s: %s", d.displayablePath(filepath.Join(libRoot, errorPath.AnchoredPath()), false, false), errorPath.GetError())
		}
		return fmt.Errorf("%s", msg.String())
	}
	return nil
}

func (d *doccurator) PrintStatus(paths []string) error {
	buckets := make(map[library.PathStatus][]library.CheckedPath)

	if len(paths) > 0 {
		d.Print(out.Verbose, "Status of %d specified %s:\n", len(paths), out.Plural(paths, "path", "paths"))
	}
	d.Print(out.Normal, "\n")

	hasChanges := false
	explicitQueryForPaths := len(paths) > 0

	processResult := func(result library.CheckedPath, relativePath string) {
		status := result.Status()
		if !status.RepresentsChange() && !explicitQueryForPaths {
			return //to hide unchanged files [when no explicit paths are queried]
		}
		buckets[status] = append(buckets[status], result)
		if status.RepresentsChange() {
			hasChanges = true
		}
	}

	if explicitQueryForPaths {
		for _, path := range paths {
			result := d.appLib.CheckFilePath(mustAbsFilepath(path), false) //explicit status query must not sacrifice correctness for performance
			processResult(result, path)
		}
	} else {
		results, _ := d.appLib.Scan([]library.PathSkipEvaluator{d.isLibFile}, nil, d.optimizedFsAccess) //full scan may optimize performance if allowed to
		for _, result := range results {
			processResult(result, mustRelFilepathToWorkingDir(filepath.Join(d.appLib.GetRoot(), result.AnchoredPath())))
		}
	}

	// special treatment to filter missing results for which a moved entry is displayed
	// note: remaining entries do not imply they have not been moved - maybe the target destination was just out of scan scope!
	if missingCount := len(buckets[library.Missing]); missingCount > 0 {
		filteredMissing := make([]library.CheckedPath, 0, missingCount)
		movedIds := make(map[document.Id]bool)
		for _, moved := range buckets[library.Moved] {
			originalRecord := moved.ReferencedDocument()
			movedIds[originalRecord.Id()] = true
		}
		for _, missing := range buckets[library.Missing] {
			lost := missing.ReferencedDocument()
			if _, wasMoved := movedIds[lost.Id()]; !wasMoved {
				filteredMissing = append(filteredMissing, missing)
			}
		}
		buckets[library.Missing] = filteredMissing
	}

	//TODO [FEATURE]: coloring

	for _, status := range []library.PathStatus{
		library.Tracked,
		library.Removed,
		library.Obsolete,
		library.Duplicate,
		library.Untracked,
		library.Touched,
		library.Moved,
		library.Modified,
		library.Missing,
		library.Error,
	} {
		bucket := buckets[status]
		if len(bucket) == 0 {
			continue //to hide empty buckets
		}

		//bucket header
		if status == library.Error {
			d.Print(out.Normal, " %s occurred:\n", out.Plural(len(bucket), "Error", "Errors")) //not on stderr because it was explicitly queried)
		} else {
			d.Print(out.Normal, " %s (%d %s)\n", status, len(bucket), out.Plural(bucket, "file", "files"))
		}

		//bucket content
		for _, result := range bucket {
			d.Print(out.Normal, "  ")
			d.Print(out.Required, "[%c] %s\n", rune(status), d.displayablePath(d.appLib.Absolutize(result.AnchoredPath()), status != library.Error, true))
			switch status {
			case library.Moved:
				originalRecord := result.ReferencedDocument()
				d.Print(out.Normal, "      previous: %s\n", d.displayablePath(d.appLib.Absolutize(originalRecord.AnchoredPath()), true, false))
			case library.Error:
				d.Print(out.Normal, "      ")
				d.Print(out.Error, "%s\n", result.GetError())
			}
		}
		d.Print(out.Normal, "\n")
	}
	if hasChanges == false && len(paths) == 0 {
		d.Print(out.Normal, " Library files in sync with all records.\n\n")
	}
	return nil
}

func (d *doccurator) SearchByIdPart(part string) (results []SearchResult) {
	partInUpper := strings.ToUpper(part)
	d.appLib.VisitAllRecords(func(doc library.Document) {
		if id := doc.Id(); strings.Contains(id.String(), partInUpper) {
			absolute := filepath.Join(d.appLib.GetRoot(), doc.AnchoredPath())
			results = append(results, SearchResult{
				Id:         id,
				Path:       mustRelFilepathToWorkingDir(absolute),
				StatusText: d.appLib.CheckFilePath(absolute, d.optimizedFsAccess).Status().String()})
		}
	})
	return
}

func (d *doccurator) GetFreeId() document.Id {
	candidate := internal.UnixTimestampNow()
	for candidate > 0 {
		id := document.Id(candidate)
		if _, exists := d.appLib.GetDocumentById(id); !exists {
			return id
		}
		candidate--
	}
	return document.MissingId
}

func (d *doccurator) isLibFile(absolute string, dir bool) bool {
	return absolute == d.libFile && !dir
}
