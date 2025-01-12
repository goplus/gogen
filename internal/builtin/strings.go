package builtin

import "strings"

func Capitalize(str string) string {
	if str == "" {
		return ""
	}
	return strings.ToUpper(str[0:1]) + str[1:]
}
