package cache

import (
	"context"
	"testing"
	"time"
)

func TestMemoryCache_SetGet(t *testing.T) {
	c := NewMemoryCache(time.Minute)
	defer c.Close()
	ctx := context.Background()

	type payload struct{ Name string }
	if err := c.Set(ctx, "k", payload{Name: "x"}); err != nil {
		t.Fatal(err)
	}

	var got payload
	if err := c.Get(ctx, "k", &got); err != nil {
		t.Fatal(err)
	}
	if got.Name != "x" {
		t.Errorf("got %+v", got)
	}
}

func TestMemoryCache_Expiry(t *testing.T) {
	c := NewMemoryCache(time.Minute)
	defer c.Close()
	ctx := context.Background()

	_ = c.SetWithTTL(ctx, "k", "v", 10*time.Millisecond)
	time.Sleep(20 * time.Millisecond)

	var s string
	if err := c.Get(ctx, "k", &s); err == nil {
		t.Fatal("expected key to be expired")
	}
	if ok, _ := c.Exists(ctx, "k"); ok {
		t.Fatal("expected key to be absent")
	}
}

func TestMemoryCache_Delete(t *testing.T) {
	c := NewMemoryCache(time.Minute)
	defer c.Close()
	ctx := context.Background()

	_ = c.Set(ctx, "k", "v")
	_ = c.Delete(ctx, "k")

	if ok, _ := c.Exists(ctx, "k"); ok {
		t.Fatal("expected key to be deleted")
	}
}
