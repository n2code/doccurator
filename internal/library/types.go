package library

import (
	"github.com/n2code/doccurator/internal/document"
)

type library struct {
	documents          map[document.Id]document.Api
	relPathActiveIndex map[string]document.Api //active paths represent non-obsolete documents
	rootPath           string                  //absolute, system-native path
}

type orderedDocuments []document.Api
type docsByRecordedAndId orderedDocuments
