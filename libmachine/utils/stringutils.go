package utils

import (
	"bytes"
	"strings"
)

// replaceChars returns a copy of the src slice with each string modified by the replacer
func ReplaceChars(src []string, replacer *strings.Replacer) []string {
	ret := make([]string, len(src))
	for i, s := range src {
		ret[i] = replacer.Replace(s)
	}
	return ret
}

// concatStrings concatenates each string in the src slice with prefix and postfix and returns a new slice
func ConcatStrings(src []string, prefix string, postfix string) []string {
	var buf bytes.Buffer
	ret := make([]string, len(src))
	for i, s := range src {
		buf.WriteString(prefix)
		buf.WriteString(s)
		buf.WriteString(postfix)
		ret[i] = buf.String()
		buf.Reset()
	}
	return ret
}
