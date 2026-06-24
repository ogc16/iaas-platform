package dashboard

import "testing"

func TestIndexHTMLNotEmpty(t *testing.T) {
	b, err := IndexHTML()
	if err != nil {
		t.Fatalf("IndexHTML returned error: %v", err)
	}
	if len(b) == 0 {
		t.Fatalf("IndexHTML returned empty html")
	}
}

