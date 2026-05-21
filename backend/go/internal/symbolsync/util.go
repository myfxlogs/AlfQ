// Package symbolsync — nil-safe JSON marshal helper.
package symbolsync

import "encoding/json"

func marshalJSON(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}
