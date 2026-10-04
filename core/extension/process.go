package extension

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/thinkerqaq/devtool/core/contract"
	"github.com/thinkerqaq/devtool/core/project"
	"github.com/thinkerqaq/devtool/protocol"
)

type ProjectProcess struct {
	Project project.Project
	Source  string
}

type describePayload struct {
	Extension Descriptor                  `json:"extension"`
	Project   contract.ProjectDescriptor `json:"project"`
}

func (p ProjectProcess) Describe(ctx context.Context) (Descriptor, contract.ProjectDescriptor, error) {
	executable, err := p.build(ctx)
	if err != nil {
		return Descriptor{}, contract.ProjectDescriptor{}, err
	}

	var payload describePayload
	if err := p.invoke(ctx, executable, protocol.MethodDescribe, nil, io.Discard, &payload); err != nil {
		return Descriptor{}, contract.ProjectDescriptor{}, err
	}
	if payload.Extension.Kind != KindProject {
		return Descriptor{}, contract.ProjectDescriptor{}, fmt.Errorf("extension %q has kind %q; expected %q", payload.Extension.ID, payload.Extension.Kind, KindProject)
	}
	if err := contract.ValidateProjectDescriptor(payload.Project); err != nil {
		return Descriptor{}, contract.ProjectDescriptor{}, fmt.Errorf("validate project extension descriptor: %w", err)
	}
	return payload.Extension, payload.Project, nil
}

func (p ProjectProcess) Execute(ctx context.Context, command string, args map[string]any, out io.Writer) error {
	executable, err := p.build(ctx)
	if err != nil {
		return err
	}
	request := protocol.ExecuteRequest{Command: command, Args: args}
	return p.invoke(ctx, executable, protocol.MethodExecute, request, out, nil)
}

func (p ProjectProcess) build(ctx context.Context) (string, error) {
	source := strings.TrimSpace(p.Source)
	if source == "" {
		return "", fmt.Errorf("project extension source is required")
	}
	cacheDir := filepath.Join(p.Project.Root, ".devtool", "cache")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", err
	}
	name := "project-provider"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	output := filepath.Join(cacheDir, name)

	cmd := exec.CommandContext(ctx, "go", "build", "-o", output, source)
	cmd.Dir = p.Project.Root
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("build project extension: %w", err)
	}
	return output, nil
}

func (p ProjectProcess) invoke(ctx context.Context, executable, method string, payload any, out io.Writer, result any) error {
	cmd := exec.CommandContext(ctx, executable)
	cmd.Dir = p.Project.Root
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}

	requestID := "1"
	var raw json.RawMessage
	if payload != nil {
		raw, err = json.Marshal(payload)
		if err != nil {
			_ = cmd.Process.Kill()
			return err
		}
	}
	if err := json.NewEncoder(stdin).Encode(protocol.Envelope{
		ID:      requestID,
		Type:    protocol.MessageRequest,
		Method:  method,
		Payload: raw,
	}); err != nil {
		_ = cmd.Process.Kill()
		return err
	}
	_ = stdin.Close()

	scanner := bufio.NewScanner(stdout)
	gotResponse := false
	for scanner.Scan() {
		var envelope protocol.Envelope
		if err := json.Unmarshal(scanner.Bytes(), &envelope); err != nil {
			_ = cmd.Process.Kill()
			return fmt.Errorf("project extension emitted invalid protocol frame: %w", err)
		}
		if envelope.ReplyTo != requestID {
			continue
		}
		switch envelope.Type {
		case protocol.MessageEvent:
			var event protocol.Event
			if err := json.Unmarshal(envelope.Payload, &event); err != nil {
				return err
			}
			if out != nil {
				fmt.Fprintln(out, event.Message)
			}
		case protocol.MessageError:
			var failure struct {
				Message string `json:"message"`
			}
			if err := json.Unmarshal(envelope.Payload, &failure); err != nil {
				return err
			}
			_ = cmd.Wait()
			return fmt.Errorf("project extension: %s", failure.Message)
		case protocol.MessageResponse:
			gotResponse = true
			if result != nil {
				if err := json.Unmarshal(envelope.Payload, result); err != nil {
					return err
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		_ = cmd.Process.Kill()
		return err
	}
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("project extension process: %w", err)
	}
	if !gotResponse {
		return fmt.Errorf("project extension exited without a response")
	}
	return nil
}
