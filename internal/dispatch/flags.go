package dispatch

// flagStr extracts a string flag value or returns the zero value.
func flagStr(flags map[string]any, key string) string {
	v, _ := flags[key].(string)
	return v
}

// flagInt extracts an int flag value or returns the zero value.
func flagInt(flags map[string]any, key string) int {
	v, _ := flags[key].(int)
	return v
}

// flagBool extracts a bool flag value or returns the zero value.
func flagBool(flags map[string]any, key string) bool {
	v, _ := flags[key].(bool)
	return v
}

// flagStrSlice extracts a string-slice flag value or returns nil.
func flagStrSlice(flags map[string]any, key string) []string {
	v, _ := flags[key].([]string)
	return v
}
