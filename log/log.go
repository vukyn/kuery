package log

import "encoding/json"

// Print with formatted
func PrettyPrint(v interface{}) string {
	res, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(res)
}
