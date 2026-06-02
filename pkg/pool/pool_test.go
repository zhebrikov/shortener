package pool

import "testing"

type testObject struct {
	value      int
	resetCalls int
}

func (o *testObject) Reset() {
	o.resetCalls++
	o.value = 0
}

func TestPool_GetReturnsCorrectType(t *testing.T) {
	p := New(func() *testObject { return &testObject{value: 42} })

	obj := p.Get()
	if obj == nil {
		t.Fatal("Get() returned nil")
	}
	if obj.value != 42 {
		t.Errorf("Get() value = %d, want 42", obj.value)
	}
}

func TestPool_PutCallsReset(t *testing.T) {
	p := New(func() *testObject { return &testObject{} })

	obj := p.Get()
	obj.value = 100
	obj.resetCalls = 0

	p.Put(obj)

	if obj.resetCalls != 1 {
		t.Errorf("Reset() calls = %d, want 1", obj.resetCalls)
	}
}

func TestPool_GetAfterPutReturnsResetObject(t *testing.T) {
	p := New(func() *testObject { return &testObject{} })

	obj := p.Get()
	obj.value = 100
	p.Put(obj)

	reused := p.Get()
	if reused.value != 0 {
		t.Errorf("Get() after Put value = %d, want 0", reused.value)
	}
}
