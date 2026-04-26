package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/pacman99/dms-pinentry/internal/assuan"
)

var (
	debug          bool
	defaultTimeout int
)

func init() {
	flag.BoolVar(&debug, "debug", false, "log Assuan I/O to stderr")
	flag.BoolVar(&debug, "d", false, "log Assuan I/O to stderr (shorthand)")
	flag.IntVar(&defaultTimeout, "timeout", 0, "default modal timeout in seconds (0 = none)")
	flag.IntVar(&defaultTimeout, "o", 0, "default modal timeout in seconds (shorthand)")
}

// prefixWriter prepends `prefix` to each newline-terminated chunk it writes,
// so debug output is line-aligned even when callers split writes oddly.
type prefixWriter struct {
	w        io.Writer
	prefix   string
	midLine  bool
}

func (p *prefixWriter) Write(b []byte) (int, error) {
	var out []byte
	for _, c := range b {
		if !p.midLine {
			out = append(out, p.prefix...)
			p.midLine = true
		}
		out = append(out, c)
		if c == '\n' {
			p.midLine = false
		}
	}
	if _, err := p.w.Write(out); err != nil {
		return 0, err
	}
	return len(b), nil
}

// Request is sent to DMS via IPC to trigger the modal.
type Request struct {
	Type        string `json:"type"`
	Socket      string `json:"socket"`
	Title       string `json:"title,omitempty"`
	Desc        string `json:"desc,omitempty"`
	Prompt      string `json:"prompt,omitempty"`
	Error       string `json:"error,omitempty"`
	OKLabel     string `json:"okLabel,omitempty"`
	CancelLabel string `json:"cancelLabel,omitempty"`
	NotOKLabel  string `json:"notOkLabel,omitempty"`
	Timeout     int    `json:"timeout,omitempty"`
	Repeat      bool   `json:"repeat,omitempty"`
}

// Response is received from DMS over the socket.
type Response struct {
	Type  string `json:"type"`
	Value string `json:"value,omitempty"`
}

func main() {
	flag.Parse()

	var stdout io.Writer = os.Stdout
	if debug {
		stdout = io.MultiWriter(os.Stdout, &prefixWriter{w: os.Stderr, prefix: "-> "})
	}

	reader := assuan.NewReader(os.Stdin)
	writer := assuan.NewWriter(stdout)

	// Initial greeting
	fmt.Fprintln(stdout, "OK Pleased to meet you")

	state := &assuan.State{Timeout: defaultTimeout}

	for {
		cmd, err := reader.ReadCommand()
		if err != nil {
			return
		}
		if debug {
			fmt.Fprintf(os.Stderr, "<- %s %s\n", cmd.Name, cmd.Param)
		}

		switch cmd.Name {
		case "GETPIN":
			handleGetPin(state, writer)
			state.Reset()

		case "CONFIRM":
			oneButton := cmd.Param == "--one-button"
			handleConfirm(state, writer, oneButton)

		case "MESSAGE":
			handleMessage(state, writer)

		case "GETINFO":
			handleGetInfo(cmd, writer)

		case "BYE":
			writer.OK("closing connection")
			return

		case "RESET":
			*state = assuan.State{Timeout: defaultTimeout}
			writer.OK("")

		case "NOP":
			writer.OK("")

		default:
			if state.ApplyCommand(cmd) {
				writer.OK("")
			} else {
				writer.OK("")
			}
		}
	}
}

func socketPath() string {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = fmt.Sprintf("/tmp/dms-pinentry-%d", os.Getuid())
	}
	var id [8]byte
	rand.Read(id[:])
	return filepath.Join(dir, fmt.Sprintf("dms-pinentry-%s.sock", hex.EncodeToString(id[:])))
}

func handleGetPin(state *assuan.State, writer *assuan.Writer) {
	resp, err := showModal("getpin", state)
	if err != nil {
		writer.Error(assuan.ErrGeneral.WithMessage(err.Error()))
		return
	}

	switch resp.Type {
	case "pin":
		writer.Data(resp.Value)
		writer.OK("")
	case "cancel":
		writer.Error(assuan.ErrCanceled)
	case "timeout":
		writer.Error(assuan.ErrTimeout)
	default:
		writer.Error(assuan.ErrGeneral.WithMessage("unexpected response"))
	}
}

func handleConfirm(state *assuan.State, writer *assuan.Writer, oneButton bool) {
	modalType := "confirm"
	if oneButton {
		modalType = "message"
	}

	resp, err := showModal(modalType, state)
	if err != nil {
		writer.Error(assuan.ErrGeneral.WithMessage(err.Error()))
		return
	}

	switch resp.Type {
	case "ok":
		writer.OK("")
	case "cancel":
		writer.Error(assuan.ErrCanceled)
	case "timeout":
		writer.Error(assuan.ErrTimeout)
	case "notok":
		writer.Error(assuan.ErrNotConfirmed)
	default:
		writer.Error(assuan.ErrGeneral.WithMessage("unexpected response"))
	}
}

func handleMessage(state *assuan.State, writer *assuan.Writer) {
	resp, err := showModal("message", state)
	if err != nil {
		writer.Error(assuan.ErrGeneral.WithMessage(err.Error()))
		return
	}

	switch resp.Type {
	case "ok":
		writer.OK("")
	case "timeout":
		writer.Error(assuan.ErrTimeout)
	default:
		writer.Error(assuan.ErrGeneral.WithMessage("unexpected response"))
	}
}

func handleGetInfo(cmd assuan.Command, writer *assuan.Writer) {
	switch cmd.Param {
	case "pid":
		writer.Data(fmt.Sprintf("%d", os.Getpid()))
		writer.OK("")
	case "version":
		writer.Data("0.1.0")
		writer.OK("")
	case "flavor":
		writer.Data("dms")
		writer.OK("")
	case "ttyinfo":
		writer.Data("")
		writer.OK("")
	default:
		writer.OK("")
	}
}

func showModal(modalType string, state *assuan.State) (*Response, error) {
	sockPath := socketPath()

	// Clean up any stale socket
	os.Remove(sockPath)

	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		return nil, fmt.Errorf("listen: %w", err)
	}
	defer func() {
		listener.Close()
		os.Remove(sockPath)
	}()

	// Set socket permissions to owner-only
	os.Chmod(sockPath, 0600)

	req := Request{
		Type:        modalType,
		Socket:      sockPath,
		Title:       state.Title,
		Desc:        state.Desc,
		Prompt:      state.Prompt,
		Error:       state.Error,
		OKLabel:     state.OKLabel,
		CancelLabel: state.CancelLabel,
		NotOKLabel:  state.NotOKLabel,
		Timeout:     state.Timeout,
		Repeat:      state.Repeat,
	}

	reqJSON, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	// Fire off DMS IPC to show modal
	ipcCmd := exec.Command("dms", "ipc", "call", "dankPinentry", "prompt", string(reqJSON))
	if err := ipcCmd.Start(); err != nil {
		return nil, fmt.Errorf("ipc: %w", err)
	}
	// Don't wait for ipc command — it returns immediately
	go ipcCmd.Wait()

	// Buffer past the modal's own timer so Accept only fails if DMS never showed up.
	const acceptBuffer = 10 * time.Second
	acceptDeadline := 60 * time.Second
	if state.Timeout > 0 {
		acceptDeadline = time.Duration(state.Timeout)*time.Second + acceptBuffer
	}
	listener.(*net.UnixListener).SetDeadline(time.Now().Add(acceptDeadline))

	conn, err := listener.Accept()
	listener.Close() // Minimizes window for a second listener
	if err != nil {
		return nil, fmt.Errorf("accept: %w", err)
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(acceptDeadline))

	decoder := json.NewDecoder(conn)
	var resp Response
	if err := decoder.Decode(&resp); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	return &resp, nil
}
