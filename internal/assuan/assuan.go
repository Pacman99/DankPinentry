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

// Error sends an ERR response.
func (w *Writer) Error(code int, msg string) error {
	_, err := fmt.Fprintf(w.w, "ERR %d %s\n", code, msg)
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

// Standard error codes used by pinentry.
const (
	ErrCanceled    = 83886179 // GPG_ERR_CANCELED
	ErrNotConfirmed = 83886194 // GPG_ERR_NOT_CONFIRMED
	ErrASS_UNKNOWN_CMD = 83886381 // GPG_ERR_ASS_UNKNOWN_CMD
	ErrGeneral     = 83886129 // GPG_ERR_GENERAL
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
