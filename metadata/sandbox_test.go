/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package metadata

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/sandbox"
	"github.com/gogo/protobuf/types"
	"github.com/opencontainers/runtime-spec/specs-go"
)

func TestSandboxStart(t *testing.T) {
	ctx, db, done := testDB(t, withSandbox("test", &mockController{}))
	defer done()

	sb := db.Sandboxes()
	store := sb["test"]

	in := sandbox.Instance{
		ID:         "1",
		Labels:     map[string]string{"test": "1"},
		Spec:       &sandbox.Spec{},
		Extensions: map[string]types.Any{},
	}

	out, err := store.Start(ctx, in)

	if err != nil {
		t.Fatal(err)
	}

	if out.ID != "1" {
		t.Fatalf("unexpected instance ID: %q", out.ID)
	}

	if out.CreatedAt.IsZero() {
		t.Fatal("creation time not assigned")
	}

	if out.UpdatedAt.IsZero() {
		t.Fatal("updated time not assigned")
	}

	// Read back
	out, err = store.Find(ctx, "1")
	if err != nil {
		t.Fatal(err)
	}

	assertEqualInstances(t, in, out)
}

func TestSandboxRollbackStart(t *testing.T) {
	ctx, db, done := testDB(t, withSandbox("test", &mockController{err: errors.New("failed to start")}))
	defer done()

	sb := db.Sandboxes()
	store := sb["test"]

	_, err := store.Start(ctx, sandbox.Instance{
		ID:     "1",
		Labels: map[string]string{"test": "1"},
		Spec:   &sandbox.Spec{},
	})

	if err == nil {
		t.Fatal("expected start error")
	}

	_, err = store.Find(ctx, "1")
	if err == nil {
		t.Fatal("should not have saved failed instance to store")
	} else if !errdefs.IsNotFound(err) {
		t.Fatal("expected 'not found' error")
	}
}

func TestSandboxStartDup(t *testing.T) {
	ctx, db, done := testDB(t, withSandbox("test", &mockController{}))
	defer done()

	sb := db.Sandboxes()
	store := sb["test"]

	in := sandbox.Instance{
		ID:     "1",
		Labels: map[string]string{"test": "1"},
		Spec:   &sandbox.Spec{},
	}

	_, err := store.Start(ctx, in)
	if err != nil {
		t.Fatal(err)
	}

	_, err = store.Start(ctx, in)
	if err == nil {
		t.Fatal("should return error on double start")
	} else if !errdefs.IsAlreadyExists(err) {
		t.Fatalf("unexpected error: %+v", err)
	}
}

func TestSandboxStop(t *testing.T) {
	ctx, db, done := testDB(t, withSandbox("test", &mockController{
		update: func(action string, instance sandbox.Instance) (sandbox.Instance, error) {
			if action == "stop" {
				instance.Labels["test"] = "updated"
			}
			return instance, nil
		},
	}))
	defer done()

	sb := db.Sandboxes()
	store := sb["test"]

	_, err := store.Start(ctx, sandbox.Instance{
		ID:     "1",
		Labels: map[string]string{"test": "1"},
		Spec:   &sandbox.Spec{},
	})

	if err != nil {
		t.Fatal(err)
	}

	err = store.Stop(ctx, "1")
	if err != nil {
		t.Fatal(err)
	}

	// Read back, make sure dabase got updated instance

	instance, err := store.Find(ctx, "1")
	if err != nil {
		t.Fatal(err)
	}

	if len(instance.Labels) != 1 {
		t.Fatal("invalid label map after stop")
	} else if instance.Labels["test"] != "updated" {
		t.Fatal("invalid labels after stop")
	}
}

func TestSandboxStopInvalid(t *testing.T) {
	ctx, db, done := testDB(t, withSandbox("test", &mockController{}))
	defer done()

	sb := db.Sandboxes()
	store := sb["test"]

	err := store.Stop(ctx, "invalid_id")
	if err == nil {
		t.Fatal("expected 'not found' error for invalid ID")
	} else if !errdefs.IsNotFound(err) {
		t.Fatalf("unexpected error %T type", err)
	}
}

func TestSandboxUpdate(t *testing.T) {
	ctx, db, done := testDB(t, withSandbox("test", &mockController{
		update: func(action string, instance sandbox.Instance) (sandbox.Instance, error) {
			if action == "update" {
				instance.Labels["lbl2"] = "updated"
				instance.Extensions["ext2"] = types.Any{
					TypeUrl: "url2",
					Value:   []byte{4, 5, 6},
				}
			}
			return instance, nil
		},
	}))
	defer done()

	sb := db.Sandboxes()
	store := sb["test"]

	_, err := store.Start(ctx, sandbox.Instance{
		ID:   "2",
		Spec: &sandbox.Spec{},
	})

	if err != nil {
		t.Fatal(err)
	}

	expectedSpec := &sandbox.Spec{
		Version:  "1.0.0",
		Hostname: "localhost",
		Linux: &specs.Linux{
			Sysctl: map[string]string{"a": "b"},
		},
	}

	out, err := store.Update(ctx, sandbox.Instance{
		ID:     "2",
		Labels: map[string]string{"lbl1": "new"},
		Spec:   expectedSpec,
		Extensions: map[string]types.Any{
			"ext1": {TypeUrl: "url1", Value: []byte{1, 2}},
		},
	}, "labels.lbl1", "extensions.ext1", "spec")

	if err != nil {
		t.Fatal(err)
	}

	expected := sandbox.Instance{
		ID:   "2",
		Spec: expectedSpec,
		Labels: map[string]string{
			"lbl1": "new",
			"lbl2": "updated",
		},
		Extensions: map[string]types.Any{
			"ext1": {TypeUrl: "url1", Value: []byte{1, 2}},
			"ext2": {TypeUrl: "url2", Value: []byte{4, 5, 6}},
		},
	}

	assertEqualInstances(t, out, expected)
}

func TestSandboxFindInvalid(t *testing.T) {
	ctx, db, done := testDB(t, withSandbox("test", &mockController{}))
	defer done()

	sb := db.Sandboxes()
	store := sb["test"]

	_, err := store.Find(ctx, "invalid_id")
	if err == nil {
		t.Fatal("expected 'not found' error for invalid ID")
	} else if !errdefs.IsNotFound(err) {
		t.Fatalf("unexpected error %T type", err)
	}
}

func TestSandboxList(t *testing.T) {
	ctx, db, done := testDB(t, withSandbox("test", &mockController{}))
	defer done()

	sb := db.Sandboxes()
	store := sb["test"]

	in := []sandbox.Instance{
		{
			ID:         "1",
			Labels:     map[string]string{"test": "1"},
			Spec:       &sandbox.Spec{},
			Extensions: map[string]types.Any{"ext": {}},
		},
		{
			ID:     "2",
			Labels: map[string]string{"test": "2"},
			Spec:   &sandbox.Spec{},
			Extensions: map[string]types.Any{"ext": {
				TypeUrl: "test",
				Value:   []byte{9},
			}},
		},
	}

	for _, inst := range in {
		_, err := store.Start(ctx, inst)
		if err != nil {
			t.Fatal(err)
		}
	}

	out, err := store.List(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(in) != len(out) {
		t.Fatalf("expected list size: %d != %d", len(in), len(out))
	}

	for i := range out {
		assertEqualInstances(t, *out[i], in[i])
	}
}

func TestSandboxListFilter(t *testing.T) {
	ctx, db, done := testDB(t, withSandbox("test", &mockController{}))
	defer done()

	sb := db.Sandboxes()
	store := sb["test"]

	in := []sandbox.Instance{
		{
			ID:         "1",
			Labels:     map[string]string{"test": "1"},
			Spec:       &sandbox.Spec{},
			Extensions: map[string]types.Any{"ext": {}},
		},
		{
			ID:     "2",
			Labels: map[string]string{"test": "2"},
			Spec:   &sandbox.Spec{},
			Extensions: map[string]types.Any{"ext": {
				TypeUrl: "test",
				Value:   []byte{9},
			}},
		},
	}

	for _, inst := range in {
		_, err := store.Start(ctx, inst)
		if err != nil {
			t.Fatal(err)
		}
	}

	out, err := store.List(ctx, `id==1`)
	if err != nil {
		t.Fatal(err)
	}

	if len(out) != 1 {
		t.Fatalf("expected list to contain 1 element, got %d", len(out))
	}

	assertEqualInstances(t, *out[0], in[0])
}

func TestSandboxDelete(t *testing.T) {
	ctx, db, done := testDB(t, withSandbox("test", &mockController{}))
	defer done()

	sb := db.Sandboxes()
	store := sb["test"]

	_, err := store.Start(ctx, sandbox.Instance{
		ID:   "3",
		Spec: &sandbox.Spec{},
	})

	if err != nil {
		t.Fatal(err)
	}

	err = store.Delete(ctx, "3")
	if err != nil {
		t.Fatal(err)
	}

	_, err = store.Find(ctx, "3")
	if !errdefs.IsNotFound(err) {
		t.Fatalf("unexpected err result: %+v != %+v", err, errdefs.ErrNotFound)
	}
}

func TestSandboxReadWrite(t *testing.T) {
	ctx, db, done := testDB(t, withSandbox("test", &mockController{}))
	defer done()

	sb := db.Sandboxes()
	store := sb["test"]

	in := sandbox.Instance{
		ID:     "1",
		Labels: map[string]string{"a": "1", "b": "2"},
		Spec: &sandbox.Spec{
			Version:  "1.0.0",
			Hostname: "localhost",
			Linux: &specs.Linux{
				Sysctl: map[string]string{"a": "b"},
			},
		},
		Extensions: map[string]types.Any{
			"ext1": {TypeUrl: "url/1", Value: []byte{1, 2, 3}},
			"ext2": {TypeUrl: "url/2", Value: []byte{3, 2, 1}},
		},
	}

	_, err := store.Start(ctx, in)
	if err != nil {
		t.Fatal(err)
	}

	out, err := store.Find(ctx, "1")
	if err != nil {
		t.Fatal(err)
	}

	assertEqualInstances(t, in, out)
}

func assertEqualInstances(t *testing.T, x, y sandbox.Instance) {
	if x.ID != y.ID {
		t.Fatalf("ids are not equal: %q != %q", x.ID, y.ID)
	}

	if !reflect.DeepEqual(x.Labels, y.Labels) {
		t.Fatalf("labels are not equal: %+v != %+v", x.Labels, y.Labels)
	}

	if !reflect.DeepEqual(x.Spec, y.Spec) {
		t.Fatalf("specs are not equal: %+v != %+v", x.Spec, y.Spec)
	}

	if !reflect.DeepEqual(x.Extensions, y.Extensions) {
		t.Fatalf("extensions are not equal: %+v != %+v", x.Extensions, y.Extensions)
	}
}

// A passthru sandbox controller for testing
type mockController struct {
	err    error
	update func(action string, instance sandbox.Instance) (sandbox.Instance, error)
}

var _ sandbox.Controller = &mockController{}

func (m *mockController) Start(ctx context.Context, instance sandbox.Instance) (sandbox.Instance, error) {
	if m.update != nil {
		return m.update("start", instance)
	}
	return instance, m.err
}

func (m *mockController) Stop(ctx context.Context, instance sandbox.Instance) (sandbox.Instance, error) {
	if m.update != nil {
		return m.update("stop", instance)
	}
	return instance, m.err
}

func (m *mockController) Update(ctx context.Context, instance sandbox.Instance, fieldpaths ...string) (sandbox.Instance, error) {
	if m.update != nil {
		return m.update("update", instance)
	}
	return instance, m.err
}

func (m *mockController) Status(ctx context.Context, instance sandbox.Instance) (sandbox.Status, error) {
	return sandbox.Status{}, m.err
}

func (m *mockController) Delete(ctx context.Context, instance sandbox.Instance) error {
	return m.err
}
