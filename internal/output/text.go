package output

func PluralS(countable interface{}) string {
	switch c := countable.(type) {
	case []string:
		if len(c) != 1 {
			return "s"
		}
	case bool:
		if c {
			return "s"
		}
	}
	return ""
}
