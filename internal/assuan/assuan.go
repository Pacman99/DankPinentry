package assuan

import (
	"bufio"
	"fmt"
	"io"
	"net/url"
	"strings"
)

const maxLineLen = 1000

// State holds accumulated pinentry dialog state from Assuan commands.
type State struct {
	Title       string
	Desc        string
	Prompt      string
	Error       string
	OKLabel     string
	CancelLabel string
	NotOKLabel  string
	KeyInfo     string
	Timeout     int
	Repeat      bool
	RepeatError string

	// OPTION values
	Grab    bool
	TTYName string
	TTYType string
	LCCtype string
	Display string
}

// Command represents a parsed Assuan command.
type Command struct {
	Name  string
	Param string
}

// Reader reads Assuan commands from a buffered reader.
type Reader struct {
	scanner *bufio.Scanner
}

// NewReader creates a new Assuan command reader.
func NewReader(r io.Reader) *Reader {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, maxLineLen), maxLineLen)
	return &Reader{scanner: scanner}
}

// ReadCommand reads the next Assuan command. Returns io.EOF when done.
func (r *Reader) ReadCommand() (Command, error) {
	if !r.scanner.Scan() {
		if err := r.scanner.Err(); err != nil {
			return Command{}, err
		}
		return Command{}, io.EOF
	}

	line := r.scanner.Text()
	name, param, _ := strings.Cut(line, " ")
	return Command{Name: strings.ToUpper(name), Param: param}, nil
}

// Writer writes Assuan responses.
type Writer struct {
	w io.Writer
}

// NewWriter creates a new Assuan response writer.
func NewWriter(w io.Writer) *Writer {
	return &Writer{w: w}
}

// OK sends an OK response.
func (w *Writer) OK(msg string) error {
	if msg != "" {
		_, err := fmt.Fprintf(w.w, "OK %s\n", msg)
		return err
	}
	_, err := fmt.Fprint(w.w, "OK\n")
	return err
}

// Error sends an ERR response. The wire format is `ERR <num> <msg> <Source>`,
// matching what libassuan emits from canonical pinentry implementations.
func (w *Writer) Error(e Error) error {
	src := e.Source.Name()
	if src != "" {
		_, err := fmt.Fprintf(w.w, "ERR %d %s <%s>\n", e.wire(), e.Message, src)
		return err
	}
	_, err := fmt.Fprintf(w.w, "ERR %d %s\n", e.wire(), e.Message)
	return err
}

// Data sends a D (data) response with percent-encoding.
func (w *Writer) Data(data string) error {
	encoded := percentEncode(data)
	_, err := fmt.Fprintf(w.w, "D %s\n", encoded)
	return err
}

// Comment sends a comment line.
func (w *Writer) Comment(msg string) error {
	_, err := fmt.Fprintf(w.w, "# %s\n", msg)
	return err
}

// Source identifies which component originated an error, matching
// libgpg-error's GPG_ERR_SOURCE_* values.
type Source uint8

const (
	SourceUnspecified Source = 0
	SourcePinentry    Source = 5
)

// Name returns the human-readable source name appended to ERR lines.
func (s Source) Name() string {
	switch s {
	case SourcePinentry:
		return "Pinentry"
	}
	return ""
}

// Error represents a GPG error. Code/Source mirror libgpg-error fields;
// Message is the human-readable text sent on the wire.
type Error struct {
	Code    uint16
	Source  Source
	Message string
}

// Error implements the error interface.
func (e Error) Error() string { return e.Message }

// WithMessage returns a copy of e with Message replaced.
func (e Error) WithMessage(msg string) Error {
	e.Message = msg
	return e
}

// wire returns the packed integer libgpg-error uses on the protocol.
func (e Error) wire() uint32 {
	return uint32(e.Source)<<24 | uint32(e.Code)
}

// Standard pinentry errors.
var (
	ErrTimeout        = Error{Code: 62, Source: SourcePinentry, Message: "Timeout"}
	ErrCanceled       = Error{Code: 99, Source: SourcePinentry, Message: "Operation cancelled"}
	ErrNotConfirmed   = Error{Code: 114, Source: SourcePinentry, Message: "Operation not confirmed"}
	ErrUnknownCommand = Error{Code: 275, Source: SourcePinentry, Message: "Unknown command"}
	ErrGeneral        = Error{Code: 49, Source: SourcePinentry, Message: "General error"}
)

// PercentDecode decodes Assuan percent-encoded strings.
func PercentDecode(s string) string {
	result, err := url.PathUnescape(strings.ReplaceAll(s, "+", "%2B"))
	if err != nil {
		return s
	}
	return result
}

func percentEncode(s string) string {
	var b strings.Builder
	for _, c := range s {
		switch {
		case c == '%':
			b.WriteString("%25")
		case c == '\n':
			b.WriteString("%0A")
		case c == '\r':
			b.WriteString("%0D")
		default:
			b.WriteRune(c)
		}
	}
	return b.String()
}

// ApplyCommand updates state based on an Assuan command.
// Returns true if the command was handled.
func (s *State) ApplyCommand(cmd Command) bool {
	param := PercentDecode(cmd.Param)

	switch cmd.Name {
	case "SETTITLE":
		s.Title = param
	case "SETDESC":
		s.Desc = param
	case "SETPROMPT":
		s.Prompt = param
	case "SETERROR":
		s.Error = param
	case "SETOK":
		s.OKLabel = param
	case "SETCANCEL":
		s.CancelLabel = param
	case "SETNOTOK":
		s.NotOKLabel = param
	case "SETKEYINFO":
		s.KeyInfo = param
	case "SETTIMEOUT":
		fmt.Sscanf(param, "%d", &s.Timeout)
	case "SETREPEAT":
		s.Repeat = true
		s.RepeatError = param
	case "OPTION":
		s.applyOption(param)
	default:
		return false
	}
	return true
}

func (s *State) applyOption(param string) {
	key, val, _ := strings.Cut(param, "=")
	switch strings.ToLower(key) {
	case "grab":
		s.Grab = true
	case "no-grab":
		s.Grab = false
	case "ttyname":
		s.TTYName = val
	case "ttytype":
		s.TTYType = val
	case "lc-ctype":
		s.LCCtype = val
	case "display":
		s.Display = val
	}
}

// Reset clears transient state (error) after a PIN attempt.
func (s *State) Reset() {
	s.Error = ""
}
