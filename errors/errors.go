package errors

import (
	"fmt"
	"strings"
)

type Position struct {
	Line   int `json:"line"`
	Column int `json:"column"`
	Offset int `json:"offset"`
}

const (
	EIllegalChar       = "E1001"
	EUnclosedString    = "E1002"
	EInvalidEscape     = "E1003"
	EInvalidNumber     = "E1004"
	EInvalidUTF8       = "E1005"
	EUnexpectedToken   = "E2001"
	ETrailingData      = "E2002"
	EMaxDepth          = "E2003"
	EExpectedKey       = "E2004"
	EDuplicateKey      = "E2005"
	ETypeMismatch      = "E3001"
	EUnknownField      = "E3002"
	ERequiredField     = "E3003"
	ECoercion          = "E3004"
	EOverflow          = "E3005"
	ESchemaType        = "E4001"
	ESchemaEnum        = "E4002"
	ESchemaRange       = "E4003"
	ESchemaPattern     = "E4004"
	ESchemaCombination = "E4005"
	ESchemaProperty    = "E4006"
	EUnsupportedType   = "E6001"
	EInvalidValue      = "E6002"
	ECycle             = "E6003"
	EInvalidPath       = "E7001"
	EUser              = "E8001"
)

// Error is the common machine-readable error used by every package.
type Error struct {
	CodeValue string
	Message   string
	Pos       Position
	JSONPath  Path
	Expected  string
	Got       string
	Context   string
	Cause     error
}

func New(code, message string) *Error {
	return &Error{CodeValue: code, Message: message, JSONPath: RootPath()}
}
func At(code, message string, pos Position, input []byte) *Error {
	return &Error{CodeValue: code, Message: message, Pos: pos, JSONPath: RootPath(), Context: MakeSnippet(input, pos.Offset)}
}
func OnPath(code, message string, path Path) *Error {
	return &Error{CodeValue: code, Message: message, JSONPath: path.Clone()}
}
func Wrap(code, message string, cause error) *Error {
	return &Error{CodeValue: code, Message: message, JSONPath: RootPath(), Cause: cause}
}
func WrapText(code, message string, cause error) *Error {
	return Wrap(code, message, fmt.Errorf("%v", cause))
}
func (e *Error) Error() string {
	var b strings.Builder
	b.WriteString("jsonx: ")
	b.WriteString(e.CodeValue)
	if e.Pos.Line > 0 {
		fmt.Fprintf(&b, " at line %d, column %d (offset %d)", e.Pos.Line, e.Pos.Column, e.Pos.Offset)
	} else if len(e.JSONPath.steps) > 0 {
		b.WriteString(" at path ")
		b.WriteString(e.JSONPath.String())
	}
	b.WriteString(": ")
	b.WriteString(e.Message)
	if e.Expected != "" {
		b.WriteString(", expected ")
		b.WriteString(e.Expected)
	}
	if e.Got != "" {
		b.WriteString(", got ")
		b.WriteString(e.Got)
	}
	return b.String()
}
func (e *Error) Code() string                 { return e.CodeValue }
func (e *Error) Position() Position           { return e.Pos }
func (e *Error) Path() Path                   { return e.JSONPath.Clone() }
func (e *Error) Snippet() string              { return e.Context }
func (e *Error) Unwrap() error                { return e.Cause }
func (e *Error) WithPath(p Path) *Error       { e.JSONPath = p.Clone(); return e }
func (e *Error) WithCause(cause error) *Error { e.Cause = cause; return e }

type ErrorList []error

func (l ErrorList) Error() string {
	parts := make([]string, 0, len(l))
	for _, err := range l {
		if err != nil {
			parts = append(parts, err.Error())
		}
	}
	return strings.Join(parts, "\n")
}
func (l ErrorList) Unwrap() []error { return []error(l) }
func MakeSnippet(input []byte, offset int) string {
	if offset < 0 {
		offset = 0
	}
	if offset > len(input) {
		offset = len(input)
	}
	start, end := offset-16, offset+32
	if start < 0 {
		start = 0
	}
	if end > len(input) {
		end = len(input)
	}
	visible := strings.NewReplacer("\n", "\\n", "\r", "\\r", "\t", "\\t").Replace(string(input[start:end]))
	prefix := strings.NewReplacer("\n", "\\n", "\r", "\\r", "\t", "\\t").Replace(string(input[start:offset]))
	if start > 0 {
		visible = "…" + visible
		prefix = "…" + prefix
	}
	if end < len(input) {
		visible += "…"
	}
	return visible + "\n" + strings.Repeat(" ", len([]rune(prefix))) + "--^"
}
