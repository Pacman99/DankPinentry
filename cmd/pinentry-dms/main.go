package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/pacman99/dms-pinentry/internal/assuan"
)

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
	reader := assuan.NewReader(os.Stdin)
	writer := assuan.NewWriter(os.Stdout)

	// Initial greeting
	fmt.Fprintln(os.Stdout, "OK Pleased to meet you")

	state := &assuan.State{}

	for {
		cmd, err := reader.ReadCommand()
		if err != nil {
			return
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
			*state = assuan.State{}
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
		writer.Error(assuan.ErrGeneral, err.Error())
		return
	}

	switch resp.Type {
	case "pin":
		writer.Data(resp.Value)
		writer.OK("")
	case "cancel":
		writer.Error(assuan.ErrCanceled, "Operation cancelled")
	default:
		writer.Error(assuan.ErrGeneral, "unexpected response")
	}
}

func handleConfirm(state *assuan.State, writer *assuan.Writer, oneButton bool) {
	modalType := "confirm"
	if oneButton {
		modalType = "message"
	}

	resp, err := showModal(modalType, state)
	if err != nil {
		writer.Error(assuan.ErrGeneral, err.Error())
		return
	}

	switch resp.Type {
	case "ok":
		writer.OK("")
	case "cancel":
		writer.Error(assuan.ErrNotConfirmed, "Not confirmed")
	case "notok":
		writer.Error(assuan.ErrNotConfirmed, "Not confirmed")
	default:
		writer.Error(assuan.ErrGeneral, "unexpected response")
	}
}

func handleMessage(state *assuan.State, writer *assuan.Writer) {
	resp, err := showModal("message", state)
	if err != nil {
		writer.Error(assuan.ErrGeneral, err.Error())
		return
	}

	switch resp.Type {
	case "ok":
		writer.OK("")
	default:
		writer.Error(assuan.ErrGeneral, "unexpected response")
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

	// Set accept timeout
	timeout := 60 * time.Second
	if state.Timeout > 0 {
		timeout = time.Duration(state.Timeout) * time.Second
	}
	listener.(*net.UnixListener).SetDeadline(time.Now().Add(timeout))

	conn, err := listener.Accept()
	listener.Close() // Minimizes window for a second listener
	if err != nil {
		return nil, fmt.Errorf("accept: %w", err)
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(timeout))

	decoder := json.NewDecoder(conn)
	var resp Response
	if err := decoder.Decode(&resp); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	return &resp, nil
}
