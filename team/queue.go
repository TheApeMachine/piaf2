package team

import (
	"sync"
)

/*
MessageKind identifies the coordination event type.
*/
type MessageKind string

const (
	TaskClaim       MessageKind = "TaskClaim"
	FileLock        MessageKind = "FileLock"
	TaskComplete    MessageKind = "TaskComplete"
	SubAgentRequest MessageKind = "SubAgentRequest"
	SubAgentResponse MessageKind = "SubAgentResponse"
)

/*
QueueMessage is a single coordination event.
*/
type QueueMessage struct {
	Kind     MessageKind `json:"kind"`
	Path     string     `json:"path,omitempty"`
	Scope    string     `json:"scope,omitempty"`
	Agent    string     `json:"agent,omitempty"`
	Task     string     `json:"task,omitempty"`
	Payload  string     `json:"payload,omitempty"`
}

/*
Queue coordinates developers and sub-agents via in-process messages.
Use Publish before editing a file (FileLock) so others can avoid conflicts.
*/
type Queue struct {
	mu     sync.Mutex
	ch     chan QueueMessage
	items  []QueueMessage
	closed bool
}

/*
NewQueue instantiates a new Queue with the given buffer size.
*/
func NewQueue(buffer int) *Queue {
	if buffer <= 0 {
		buffer = 64
	}

	return &Queue{
		ch:    make(chan QueueMessage, buffer),
		items: make([]QueueMessage, 0, buffer),
	}
}

/*
Publish sends a message to the queue.
*/
func (queue *Queue) Publish(msg QueueMessage) {
	queue.mu.Lock()
	queue.items = append(queue.items, msg)
	queue.mu.Unlock()

	select {
	case queue.ch <- msg:
	default:
	}
}

/*
Consume receives the next message, blocking until one is available.
Returns false when the queue is closed.
*/
func (queue *Queue) Consume() (QueueMessage, bool) {
	msg, ok := <-queue.ch
	return msg, ok
}

/*
Snapshot returns all messages so far for conflict checking.
*/
func (queue *Queue) Snapshot() []QueueMessage {
	queue.mu.Lock()
	defer queue.mu.Unlock()

	return append([]QueueMessage(nil), queue.items...)
}

/*
HasFileLock returns true if path (or a parent) is locked by another agent.
*/
func (queue *Queue) HasFileLock(path string, excludeAgent string) bool {
	queue.mu.Lock()
	defer queue.mu.Unlock()

	for _, msg := range queue.items {
		if msg.Kind != FileLock || msg.Agent == excludeAgent {
			continue
		}
		if msg.Path == path || containsPath(msg.Path, path) {
			return true
		}
	}

	return false
}

func containsPath(locked string, candidate string) bool {
	if locked == "" || candidate == "" {
		return false
	}

	return len(candidate) > len(locked) &&
		candidate[:len(locked)] == locked &&
		(candidate[len(locked)] == '/' || candidate[len(locked)] == '\\')
}

/*
Close closes the queue channel. Do not Publish after Close.
Safe to call multiple times.
*/
func (queue *Queue) Close() error {
	queue.mu.Lock()
	defer queue.mu.Unlock()

	if queue.closed {
		return nil
	}

	queue.closed = true
	close(queue.ch)
	return nil
}

