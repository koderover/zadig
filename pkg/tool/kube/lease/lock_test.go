package lease

import "testing"

func TestLockCheck(t *testing.T) {
	lock := &Lock{
		lostCh: make(chan struct{}),
	}
	if err := lock.Check(); err != nil {
		t.Fatalf("unexpected lock check error: %v", err)
	}

	lock.markLost(errLeaseLost)
	if err := lock.Check(); err != errLeaseLost {
		t.Fatalf("expected %v, got %v", errLeaseLost, err)
	}
}

func TestLockMarkLostIsIdempotent(t *testing.T) {
	lock := &Lock{
		lostCh: make(chan struct{}),
	}

	lock.markLost(errLeaseLost)
	lock.markLost(errLeaseLost)

	if err := lock.Err(); err != errLeaseLost {
		t.Fatalf("expected %v, got %v", errLeaseLost, err)
	}
}
