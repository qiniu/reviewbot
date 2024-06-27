package lintersutil

// LimitJoin joins the strings in str with a newline separator until the length of the result is greater than length.
func LimitJoin(str []string, length int) string {
	var result string
	for _, s := range str {
		if len(result)+len(s) > length {
			break
		}
		result += s + "\n"
	}
	return result
}
