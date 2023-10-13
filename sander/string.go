package sander

func Pluralize(text string, num int) string {
	if num == 1 {
		return text
	}
	return text + "s"
}
