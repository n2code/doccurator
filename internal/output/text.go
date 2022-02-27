package output

func Plural(countable interface{}, singular string, plural string) string {
	switch c := countable.(type) {
	case []string:
		if len(c) != 1 {
			return plural
		}
	case bool:
		if c {
			return plural
		}
	}
	return singular
}
