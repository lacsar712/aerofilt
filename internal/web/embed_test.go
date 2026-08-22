package web

import "testing"

func TestHandlerNonNil(t *testing.T) {
	h := Handler()
	if h == nil {
		t.Fatal("handler nil")
	}
}
