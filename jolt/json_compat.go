package jolt

import "encoding/json"

func MarshalJSONCompat(v any, indent bool) ([]byte, error) {
    if indent {
        return json.MarshalIndent(v, "", "  ")
    }
    return json.Marshal(v)
}

// UnmarshalJSONWithComments parses JSON that may include // and /* */ comments.
func UnmarshalJSONWithComments(data []byte, v any) error {
    clean := StripJSONComments(data)
    return json.Unmarshal(clean, v)
}
