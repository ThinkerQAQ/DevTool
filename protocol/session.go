package protocol

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
)

type RequestHandler func(context.Context, Envelope) (any, error)

type Session struct {
	encoder *json.Encoder
	sendMu  sync.Mutex

	pendingMu sync.Mutex
	pending   map[string]chan Envelope
	nextID    atomic.Uint64

	activeMu sync.Mutex
	active   map[string]context.CancelFunc

	handler  RequestHandler
	handlers sync.WaitGroup
	done     chan struct{}

	errMu sync.Mutex
	err   error
}

func NewSession(in io.Reader, out io.Writer, handler RequestHandler) *Session {
	session := &Session{
		encoder: json.NewEncoder(out),
		pending: map[string]chan Envelope{},
		active:  map[string]context.CancelFunc{},
		handler: handler,
		done:    make(chan struct{}),
	}
	go session.readLoop(in)
	return session
}

func (s *Session) Call(ctx context.Context, method string, payload any, onEvent func(Event), result any) error {
	if method == "" {
		return errors.New("rpc method is required")
	}

	raw, err := marshalPayload(payload)
	if err != nil {
		return err
	}

	id := fmt.Sprintf("%d", s.nextID.Add(1))
	ch := make(chan Envelope, 8)
	s.pendingMu.Lock()
	s.pending[id] = ch
	s.pendingMu.Unlock()
	defer func() {
		s.pendingMu.Lock()
		delete(s.pending, id)
		s.pendingMu.Unlock()
	}()

	if err := s.send(Envelope{
		ID:      id,
		Type:    MessageRequest,
		Method:  method,
		Payload: raw,
	}); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			_ = s.send(Envelope{
				ReplyTo: id,
				Type:    MessageCancel,
			})
			return ctx.Err()
		case <-s.done:
			if err := s.sessionError(); err != nil {
				return err
			}
			return io.EOF
		case envelope := <-ch:
			switch envelope.Type {
			case MessageEvent:
				if onEvent != nil {
					var event Event
					if err := json.Unmarshal(envelope.Payload, &event); err != nil {
						return err
					}
					onEvent(event)
				}
			case MessageError:
				var failure struct {
					Message string `json:"message"`
				}
				if err := json.Unmarshal(envelope.Payload, &failure); err != nil {
					return err
				}
				if failure.Message == "" {
					failure.Message = "remote call failed"
				}
				return errors.New(failure.Message)
			case MessageResponse:
				if result != nil && len(envelope.Payload) != 0 {
					if err := json.Unmarshal(envelope.Payload, result); err != nil {
						return err
					}
				}
				return nil
			default:
				return fmt.Errorf("unexpected rpc response type %q", envelope.Type)
			}
		}
	}
}

func (s *Session) Event(replyTo string, event Event) error {
	if replyTo == "" {
		return errors.New("event reply_to is required")
	}
	raw, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return s.send(Envelope{
		ReplyTo: replyTo,
		Type:    MessageEvent,
		Payload: raw,
	})
}

func (s *Session) Wait() error {
	<-s.done
	return s.sessionError()
}

func (s *Session) readLoop(in io.Reader) {
	scanner := bufio.NewScanner(in)
	for scanner.Scan() {
		var envelope Envelope
		if err := json.Unmarshal(scanner.Bytes(), &envelope); err != nil {
			s.setError(fmt.Errorf("decode rpc frame: %w", err))
			return
		}

		if envelope.Type == MessageRequest {
			if s.handler == nil {
				_ = s.replyError(envelope.ID, errors.New("rpc requests are not supported"))
				continue
			}
			requestCtx, cancel := context.WithCancel(context.Background())
			s.registerActive(envelope.ID, cancel)
			s.handlers.Add(1)
			go func(request Envelope) {
				defer s.handlers.Done()
				defer cancel()
				defer s.removeActive(request.ID)
				s.handleRequest(requestCtx, request)
			}(envelope)
			continue
		}

		if envelope.Type == MessageCancel {
			if envelope.ReplyTo == "" {
				s.setError(errors.New("rpc cancel frame is missing reply_to"))
				return
			}
			s.cancelActive(envelope.ReplyTo)
			continue
		}

		if envelope.ReplyTo == "" {
			s.setError(fmt.Errorf("rpc %s frame is missing reply_to", envelope.Type))
			return
		}

		s.pendingMu.Lock()
		ch := s.pending[envelope.ReplyTo]
		s.pendingMu.Unlock()
		if ch != nil {
			ch <- envelope
		}
	}

	if err := scanner.Err(); err != nil {
		s.setError(err)
	}
	s.cancelAllActive()
	s.handlers.Wait()
	close(s.done)
}

func (s *Session) handleRequest(ctx context.Context, request Envelope) {
	result, err := s.handler(ctx, request)
	if err != nil {
		_ = s.replyError(request.ID, err)
		return
	}
	raw, err := marshalPayload(result)
	if err != nil {
		_ = s.replyError(request.ID, err)
		return
	}
	_ = s.send(Envelope{
		ReplyTo: request.ID,
		Type:    MessageResponse,
		Payload: raw,
	})
}

func (s *Session) replyError(replyTo string, err error) error {
	raw, marshalErr := json.Marshal(map[string]string{"message": err.Error()})
	if marshalErr != nil {
		return marshalErr
	}
	return s.send(Envelope{
		ReplyTo: replyTo,
		Type:    MessageError,
		Payload: raw,
	})
}

func (s *Session) registerActive(id string, cancel context.CancelFunc) {
	s.activeMu.Lock()
	defer s.activeMu.Unlock()
	s.active[id] = cancel
}

func (s *Session) removeActive(id string) {
	s.activeMu.Lock()
	defer s.activeMu.Unlock()
	delete(s.active, id)
}

func (s *Session) cancelActive(id string) {
	s.activeMu.Lock()
	cancel := s.active[id]
	s.activeMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (s *Session) cancelAllActive() {
	s.activeMu.Lock()
	cancels := make([]context.CancelFunc, 0, len(s.active))
	for _, cancel := range s.active {
		cancels = append(cancels, cancel)
	}
	s.activeMu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
}

func (s *Session) send(envelope Envelope) error {
	s.sendMu.Lock()
	defer s.sendMu.Unlock()
	return s.encoder.Encode(envelope)
}

func (s *Session) setError(err error) {
	s.errMu.Lock()
	defer s.errMu.Unlock()
	if s.err == nil {
		s.err = err
	}
}

func (s *Session) sessionError() error {
	s.errMu.Lock()
	defer s.errMu.Unlock()
	return s.err
}

func marshalPayload(payload any) (json.RawMessage, error) {
	if payload == nil {
		return nil, nil
	}
	if raw, ok := payload.(json.RawMessage); ok {
		return raw, nil
	}
	return json.Marshal(payload)
}
