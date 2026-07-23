package queue

import (
	"strings"
	"testing"
	"time"
)

func TestMemoryQueue_PublishConsume(t *testing.T) {
	q := NewMemoryQueue(4)
	defer q.Close()

	if err := q.PublishTask(&VoiceTask{TaskID: "t1"}); err != nil {
		t.Fatal(err)
	}

	got := make(chan string, 1)
	go q.Consume("", 2, func(b []byte) error {
		got <- string(b)
		return nil
	})

	select {
	case body := <-got:
		if !strings.Contains(body, "t1") {
			t.Errorf("unexpected body: %s", body)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for task")
	}
}

func TestMemoryQueue_ConsumeStopsOnClose(t *testing.T) {
	q := NewMemoryQueue(1)

	done := make(chan struct{})
	go func() {
		q.Consume("", 3, func([]byte) error { return nil })
		close(done)
	}()

	q.Close()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Consume did not stop after Close")
	}
}
