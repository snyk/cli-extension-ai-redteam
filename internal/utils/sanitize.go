package utils

const maxBodyLen = 200

// TruncateBody truncates an API response body so it can be safely
// included in error messages without leaking large amounts of
// potentially sensitive data.
func TruncateBody(body []byte) string {
	if len(body) <= maxBodyLen {
		return string(body)
	}
	return string(body[:maxBodyLen]) + "... (truncated)"
}
