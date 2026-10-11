package obs

import (
	"fmt"
	"testing"
)

func TestRejectionsKeepsLastNewestFirst(t *testing.T) {
	r := NewRejections(3)
	for i := 1; i <= 5; i++ {
		r.Add(Rejection{EventID: fmt.Sprint(i)})
	}
	got := r.Last(0)
	if len(got) != 3 || got[0].EventID != "5" || got[2].EventID != "3" {
		t.Fatalf("got %+v", got)
	}
	if len(r.Last(2)) != 2 {
		t.Fatal("limit not applied")
	}
}
