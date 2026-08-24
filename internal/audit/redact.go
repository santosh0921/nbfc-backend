package audit

import (
	"encoding/json"
	"strings"
)

// sensitiveKeys are field names stripped out of any before/after snapshot
// before it's serialized into the audit log — the log itself must not
// become a second place secrets/PII can leak from. Matched
// case-insensitively against JSON keys (including nested objects).
var sensitiveKeys = map[string]bool{
	"password":       true,
	"passwordhash":   true,
	"password_hash":  true,
	"otp":            true,
	"mpin":           true,
	"mpinhash":       true,
	"token":          true,
	"accesstoken":    true,
	"refreshtoken":   true,
	"refresh_token":  true,
	"jwtsecret":      true,
	"secret":         true,
	"panencrypted":   true,
	"pan_encrypted":  true,
	"pannumber":      true,
	"pan_number":     true,
	"aadhaarnumber":  true,
	"aadhaar_number": true,
	"aadhaarhash":    true,
	"databases64":    true,
	"data_base64":    true,
	"signaturepng":   true,
}

const redacted = "[REDACTED]"

// Snapshot marshals v to JSON with sensitive fields replaced by
// "[REDACTED]", for use as an audit log's BeforeValue/AfterValue. Accepts
// nil (returns ""). Marshal failures return "" rather than panicking —
// an audit row missing its snapshot is far better than one that crashes
// the request it's supposed to be recording.
func Snapshot(v interface{}) string {
	if v == nil {
		return ""
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	var generic interface{}
	if err := json.Unmarshal(raw, &generic); err != nil {
		return string(raw)
	}
	redactValue(generic)
	out, err := json.Marshal(generic)
	if err != nil {
		return ""
	}
	return string(out)
}

func redactValue(v interface{}) {
	switch t := v.(type) {
	case map[string]interface{}:
		for k, val := range t {
			if sensitiveKeys[strings.ToLower(k)] {
				t[k] = redacted
				continue
			}
			redactValue(val)
		}
	case []interface{}:
		for _, item := range t {
			redactValue(item)
		}
	}
}
