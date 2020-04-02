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
	"encoding/json"
	"strings"
	"time"

	"github.com/gogo/protobuf/types"
	runtime "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"go.etcd.io/bbolt"

	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/metadata/boltutil"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/sandbox"
)

type sandboxStore struct {
	ctrl sandbox.Controller
	name string
	db   *DB
}

var _ sandbox.Store = &sandboxStore{}

func newSandbox(db *DB, name string, ctrl sandbox.Controller) *sandboxStore {
	return &sandboxStore{
		ctrl: ctrl,
		name: name,
		db:   db,
	}
}

func (s *sandboxStore) Start(ctx context.Context, instance sandbox.Instance) (sandbox.Instance, error) {
	ns, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return sandbox.Instance{}, err
	}

	var (
		id  = instance.ID
		now = time.Now().UTC()
		ret sandbox.Instance
	)

	if id == "" {
		return sandbox.Instance{}, errors.Wrap(errdefs.ErrInvalidArgument, "empty sandbox ID")
	}

	if err := update(ctx, s.db, func(tx *bbolt.Tx) error {
		if getSandboxBucket(tx, ns, s.name, id) != nil {
			return errdefs.ErrAlreadyExists
		}

		in := sandbox.Instance{
			ID:         id,
			Spec:       instance.Spec,
			Labels:     instance.Labels,
			CreatedAt:  now,
			UpdatedAt:  now,
			Extensions: instance.Extensions,
		}

		out, err := s.ctrl.Start(ctx, in)
		if err != nil {
			return err
		}

		if in.ID != out.ID {
			return errors.Wrapf(errdefs.ErrFailedPrecondition, "controller should preserve instance ID")
		}

		if in.CreatedAt != out.CreatedAt {
			return errors.Wrapf(errdefs.ErrFailedPrecondition, "controller should preserve creation time")
		}

		bucket, err := createSandboxBucket(tx, ns, s.name, id)
		if err != nil {
			return err
		}

		if err := s.write(bucket, &out); err != nil {
			return err
		}

		ret = out
		return nil
	}); err != nil {
		return sandbox.Instance{}, err
	}

	return ret, nil
}

func (s *sandboxStore) Stop(ctx context.Context, id string) error {
	ns, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return err
	}

	if err := update(ctx, s.db, func(tx *bbolt.Tx) error {
		bucket := getSandboxBuckets(tx, ns, s.name)
		if bucket == nil {
			return errors.Wrap(errdefs.ErrNotFound, "no sandbox buckets")
		}

		in, err := s.read(bucket, []byte(id))
		if err != nil {
			return err
		}

		out, err := s.ctrl.Stop(ctx, *in)
		if err != nil {
			return err
		}

		if in.ID != out.ID {
			return errors.Wrapf(errdefs.ErrFailedPrecondition, "controller should preserve instance ID")
		}

		if in.CreatedAt != out.CreatedAt {
			return errors.Wrapf(errdefs.ErrFailedPrecondition, "controller should preserve creation time")
		}

		err = s.write(bucket, &out)
		if err != nil {
			return err
		}

		return nil
	}); err != nil {
		return err
	}

	return nil
}

func (s *sandboxStore) Update(ctx context.Context, instance sandbox.Instance, fieldpaths ...string) (sandbox.Instance, error) {
	ns, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return sandbox.Instance{}, err
	}

	var (
		ret sandbox.Instance
	)

	if err := update(ctx, s.db, func(tx *bbolt.Tx) error {
		bucket := getSandboxBuckets(tx, ns, s.name)
		if bucket == nil {
			return errors.Wrap(errdefs.ErrNotFound, "no sandbox buckets")
		}

		local, err := s.read(bucket, []byte(instance.ID))
		if err != nil {
			return err
		}

		for _, path := range fieldpaths {
			if strings.HasPrefix(path, "labels.") {
				if local.Labels == nil {
					local.Labels = map[string]string{}
				}

				key := strings.TrimPrefix(path, "labels.")
				local.Labels[key] = instance.Labels[key]
				continue
			} else if strings.HasPrefix(path, "extensions.") {
				if local.Extensions == nil {
					local.Extensions = map[string]types.Any{}
				}

				key := strings.TrimPrefix(path, "extensions.")
				local.Extensions[key] = instance.Extensions[key]
				continue
			}

			switch path {
			case "labels":
				local.Labels = instance.Labels
			case "extensions":
				local.Extensions = instance.Extensions
			case "spec":
				local.Spec = instance.Spec
			default:
				return errors.Wrapf(errdefs.ErrInvalidArgument, "cannot update %q field on sandbox %q", path, instance.ID)
			}
		}

		out, err := s.ctrl.Update(ctx, *local, fieldpaths...)
		if err != nil {
			return err
		}

		if local.ID != out.ID {
			return errors.Wrapf(errdefs.ErrFailedPrecondition, "controller should preserve instance ID")
		}

		if local.CreatedAt != out.CreatedAt {
			return errors.Wrapf(errdefs.ErrFailedPrecondition, "controller should preserve creation time")
		}

		out.UpdatedAt = time.Now().UTC()

		if err := s.write(bucket, &out); err != nil {
			return err
		}

		ret = out
		return nil
	}); err != nil {
		return sandbox.Instance{}, err
	}

	return ret, nil
}

func (s *sandboxStore) Status(ctx context.Context, id string) (sandbox.Status, error) {
	_, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return sandbox.Status{}, err
	}

	instance, err := s.Find(ctx, id)
	if err != nil {
		return sandbox.Status{}, err
	}

	return s.ctrl.Status(ctx, instance)
}

func (s *sandboxStore) Find(ctx context.Context, id string) (sandbox.Instance, error) {
	ns, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return sandbox.Instance{}, err
	}

	var (
		inst sandbox.Instance
	)

	if err := view(ctx, s.db, func(tx *bbolt.Tx) error {
		bucket := getSandboxBuckets(tx, ns, s.name)
		if bucket == nil {
			return errors.Wrap(errdefs.ErrNotFound, "no sandbox buckets")
		}

		out, err := s.read(bucket, []byte(id))
		if err != nil {
			return err
		}

		inst = *out
		return nil
	}); err != nil {
		return sandbox.Instance{}, err
	}

	return inst, nil
}

func (s *sandboxStore) List(ctx context.Context, filter ...string) ([]*sandbox.Instance, error) {
	ns, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return nil, err
	}

	var (
		list []*sandbox.Instance
	)

	if err := view(ctx, s.db, func(tx *bbolt.Tx) error {
		bucket := getSandboxBuckets(tx, ns, s.name)
		if bucket == nil {
			return errors.Wrap(errdefs.ErrNotFound, "not sandbox buckets")
		}

		if err := bucket.ForEach(func(k, v []byte) error {
			info, err := s.read(bucket, k)
			if err != nil {
				return errors.Wrapf(err, "failed to read bucket %q", string(k))
			}

			list = append(list, info)
			return nil
		}); err != nil {
			return err
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return list, nil
}

func (s *sandboxStore) Delete(ctx context.Context, id string) error {
	ns, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return err
	}

	if err := update(ctx, s.db, func(tx *bbolt.Tx) error {
		buckets := getSandboxBuckets(tx, ns, s.name)
		if buckets == nil {
			return errors.Wrap(errdefs.ErrNotFound, "no sandbox buckets")
		}

		instance, err := s.read(buckets, []byte(id))
		if err != nil {
			return err
		}

		if err := buckets.DeleteBucket([]byte(id)); err != nil {
			return errors.Wrapf(err, "failed to delete bucket %q", id)
		}

		if err := s.ctrl.Delete(ctx, *instance); err != nil {
			return errors.Wrapf(err, "failed to delete sandbox %q", id)
		}

		return nil
	}); err != nil {
		return err
	}

	return nil
}

func (s *sandboxStore) read(parent *bbolt.Bucket, id []byte) (*sandbox.Instance, error) {
	var (
		inst sandbox.Instance
		err  error
	)

	bucket := parent.Bucket(id)
	if bucket == nil {
		return nil, errors.Wrapf(errdefs.ErrNotFound, "bucket %q not found", id)
	}

	inst.ID = string(id)

	inst.Labels, err = boltutil.ReadLabels(bucket)
	if err != nil {
		return nil, err
	}

	if err := boltutil.ReadTimestamps(bucket, &inst.CreatedAt, &inst.UpdatedAt); err != nil {
		return nil, err
	}

	specData := bucket.Get([]byte("runtime_spec"))
	if specData != nil {
		runtimeSpec := runtime.Spec{}
		if err := json.Unmarshal(specData, &runtimeSpec); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal runtime spec")
		}
		inst.Spec = &runtimeSpec
	}

	inst.Extensions, err = boltutil.ReadExtensions(bucket)
	if err != nil {
		return nil, err
	}

	return &inst, nil
}

func (s *sandboxStore) write(bucket *bbolt.Bucket, info *sandbox.Instance) error {
	if err := bucket.Put([]byte("id"), []byte(info.ID)); err != nil {
		return err
	}

	if err := boltutil.WriteTimestamps(bucket, info.CreatedAt, info.UpdatedAt); err != nil {
		return err
	}

	if err := boltutil.WriteLabels(bucket, info.Labels); err != nil {
		return err
	}

	if err := boltutil.WriteExtensions(bucket, info.Extensions); err != nil {
		return err
	}

	spec, err := json.Marshal(info.Spec)
	if err != nil {
		return errors.Wrap(err, "failed to marshal runtime spec")
	}

	if err := bucket.Put([]byte("runtime_spec"), spec); err != nil {
		return err
	}

	return nil
}
