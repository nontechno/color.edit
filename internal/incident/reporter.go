package incident

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

type Reporter struct {
	Dir     string
	Backend string
}

type report struct {
	ID        string            `json:"id"`
	Timestamp time.Time         `json:"timestamp"`
	Backend   string            `json:"backend,omitempty"`
	Operation string            `json:"operation"`
	Error     string            `json:"error"`
	Context   map[string]string `json:"context,omitempty"`
	Stack     string            `json:"stack,omitempty"`
	Runtime   runtimeInfo       `json:"runtime"`
}

type runtimeInfo struct {
	GoVersion string `json:"go_version"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
}

// Record writes a diagnostic report without connection strings, credentials,
// queries, values, or user names. The returned string is safe for status text.
func (r *Reporter) Record(operation string, err error, stack []byte, context map[string]string) string {
	if err == nil {
		return ""
	}
	id := newID()
	now := time.Now().UTC()
	cleanContext := make(map[string]string, len(context))
	for key, value := range context {
		cleanContext[sanitize(key)] = sanitize(value)
	}
	payload := report{
		ID:        id,
		Timestamp: now,
		Backend:   sanitize(r.Backend),
		Operation: sanitize(operation),
		Error:     sanitize(err.Error()),
		Context:   cleanContext,
		Stack:     sanitize(string(stack)),
		Runtime: runtimeInfo{
			GoVersion: runtime.Version(),
			OS:        runtime.GOOS,
			Arch:      runtime.GOARCH,
		},
	}
	if len(payload.Error) > 16*1024 {
		payload.Error = payload.Error[:16*1024] + "…"
	}
	if err := os.MkdirAll(r.Dir, 0o700); err != nil {
		return "incident " + id + " (report unavailable)"
	}
	name := fmt.Sprintf("%s_%s.json", now.Format("20060102T150405.000000000Z"), id)
	path := filepath.Join(r.Dir, name)
	temporary, err := os.CreateTemp(r.Dir, ".incident-*")
	if err != nil {
		return "incident " + id + " (report unavailable)"
	}
	temporaryPath := temporary.Name()
	written := false
	defer func() {
		_ = temporary.Close()
		if !written {
			_ = os.Remove(temporaryPath)
		}
	}()
	_ = temporary.Chmod(0o600)
	encoder := json.NewEncoder(temporary)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(payload); err != nil {
		return "incident " + id + " (report unavailable)"
	}
	if err := temporary.Sync(); err != nil {
		return "incident " + id + " (report unavailable)"
	}
	if err := temporary.Close(); err != nil {
		return "incident " + id + " (report unavailable)"
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return "incident " + id + " (report unavailable)"
	}
	written = true
	return name
}

var (
	secretAssignment = regexp.MustCompile(`(?i)(password|passwd|pwd|token|secret)(\s*[:=]\s*)([^\s&,;]+)`)
	urlUserInfo      = regexp.MustCompile(`([A-Za-z][A-Za-z0-9+.-]*://)[^/@\s]+@`)
	mysqlUserInfo    = regexp.MustCompile(`[^\s:]+:[^@\s]+@(tcp|unix)\(`)
	unixHomeUser     = regexp.MustCompile(`(?i)(/(?:home|users)/)[^/\s]+`)
	windowsHomeUser  = regexp.MustCompile(`(?i)([A-Za-z]:\\Users\\)[^\\\s]+`)
)

func sanitize(value string) string {
	value = secretAssignment.ReplaceAllString(value, `${1}${2}[redacted]`)
	value = urlUserInfo.ReplaceAllString(value, `${1}[redacted]@`)
	value = mysqlUserInfo.ReplaceAllString(value, `[redacted]@$1(`)
	value = unixHomeUser.ReplaceAllString(value, `${1}[redacted]`)
	value = windowsHomeUser.ReplaceAllString(value, `${1}[redacted]`)
	return strings.ReplaceAll(value, "\x00", "\\x00")
}

// Redact removes common credential forms from user-visible error text.
func Redact(value string) string { return sanitize(value) }

func (r *Reporter) Redact(value string) string { return sanitize(value) }

func newID() string {
	buffer := make([]byte, 6)
	if _, err := rand.Read(buffer); err == nil {
		return hex.EncodeToString(buffer)
	}
	return fmt.Sprintf("%x", time.Now().UnixNano())
}
