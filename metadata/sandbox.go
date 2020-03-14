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
	"time"

	runtime "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"go.etcd.io/bbolt"

	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/metadata/boltutil"
	"github.com/containerd/containerd/namespaces"
	api "github.com/containerd/containerd/sandbox"
)

type sandbox struct {
	srv  api.Service
	name string
	db   *DB
}

var _ api.Store = &sandbox{}

func newSandbox(db *DB, name string, srv api.Service) *sandbox {
	return &sandbox{
		srv:  srv,
		name: name,
		db:   db,
	}
}

func (s *sandbox) Start(ctx context.Context, create *api.CreateInfo) (*api.Info, error) {
	ns, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return nil, err
	}

	if create.ID == "" {
		return nil, errors.Wrap(errdefs.ErrInvalidArgument, "empty sandbox ID")
	}

	var (
		now  = time.Now().UTC()
		resp *api.Info
	)

	info := api.Info{
		ID:          create.ID,
		RuntimeSpec: create.RuntimeSpec,
		Labels:      create.Labels,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := update(ctx, s.db, func(tx *bbolt.Tx) error {
		if getSandboxBucket(tx, ns, s.name, info.ID) != nil {
			return errdefs.ErrAlreadyExists
		}

		resp, err = s.srv.Start(ctx, create)
		if err != nil {
			return errors.Wrap(err, "failed to start sandbox")
		}

		info.Annotations = resp.Annotations

		bucket, err := createSandboxBucket(tx, ns, s.name, info.ID)
		if err != nil {
			return err
		}

		if err := s.writeInfo(bucket, &info); err != nil {
			return err
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return resp, nil
}

func (s *sandbox) Stop(ctx context.Context, id string) error {
	_, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return err
	}

	return s.srv.Stop(ctx, id)
}

func (s *sandbox) Update(ctx context.Context, createInfo *api.CreateInfo, fieldpaths ...string) error {
	return errdefs.ErrNotImplemented
}

func (s *sandbox) Status(ctx context.Context, id string) (api.Status, error) {
	_, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return api.Status{}, err
	}

	return s.srv.Status(ctx, id)
}

func (s *sandbox) Info(ctx context.Context, id string) (*api.Info, error) {
	ns, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return nil, err
	}

	var (
		info *api.Info
	)

	if err := view(ctx, s.db, func(tx *bbolt.Tx) error {
		bucket := getSandboxBuckets(tx, ns, s.name)
		if bucket == nil {
			return errors.Wrap(errdefs.ErrNotFound, "no sandbox buckets")
		}

		info, err = s.readInfo(bucket, []byte(id))
		if err != nil {
			return err
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return info, nil
}

func (s *sandbox) List(ctx context.Context, filter ...string) ([]*api.Info, error) {
	ns, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return nil, err
	}

	var (
		list []*api.Info
	)

	if err := view(ctx, s.db, func(tx *bbolt.Tx) error {
		bucket := getSandboxBuckets(tx, ns, s.name)
		if bucket == nil {
			return errors.Wrap(errdefs.ErrNotFound, "not sandbox buckets")
		}

		if err := bucket.ForEach(func(k, v []byte) error {
			info, err := s.readInfo(bucket, k)
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

func (s *sandbox) Delete(ctx context.Context, id string) error {
	ns, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return err
	}

	if err := update(ctx, s.db, func(tx *bbolt.Tx) error {
		buckets := getSandboxBuckets(tx, ns, s.name)
		if buckets == nil {
			return errors.Wrap(errdefs.ErrNotFound, "no sandbox buckets")
		}

		if err := buckets.DeleteBucket([]byte(id)); err != nil {
			return errors.Wrapf(err, "failed to delete bucket %q", id)
		}

		if err := s.srv.Delete(ctx, id); err != nil {
			return errors.Wrapf(err, "failed to delete sandbox %q", id)
		}

		return nil
	}); err != nil {
		return err
	}

	return nil
}

func (s *sandbox) readInfo(parent *bbolt.Bucket, id []byte) (*api.Info, error) {
	var (
		info api.Info
		err  error
	)

	bucket := parent.Bucket(id)
	if bucket == nil {
		return nil, errors.Wrapf(errdefs.ErrNotFound, "bucket %q not found", id)
	}

	info.ID = string(id)

	info.Labels, err = boltutil.ReadLabels(bucket)
	if err != nil {
		return nil, err
	}

	info.Annotations, err = boltutil.ReadAnnotations(bucket)
	if err != nil {
		return nil, err
	}

	if err := boltutil.ReadTimestamps(bucket, &info.CreatedAt, &info.UpdatedAt); err != nil {
		return nil, err
	}

	specData := bucket.Get([]byte("runtime_spec"))
	if specData != nil {
		runtimeSpec := runtime.Spec{}
		if err := json.Unmarshal(specData, &runtimeSpec); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal runtime spec")
		}
		info.RuntimeSpec = &runtimeSpec
	}

	info.Extensions, err = boltutil.ReadExtensions(bucket)
	if err != nil {
		return nil, err
	}

	return &info, nil
}

func (s *sandbox) writeInfo(bucket *bbolt.Bucket, info *api.Info) error {
	if err := bucket.Put([]byte("id"), []byte(info.ID)); err != nil {
		return err
	}

	if err := boltutil.WriteTimestamps(bucket, info.CreatedAt, info.UpdatedAt); err != nil {
		return err
	}

	if err := boltutil.WriteLabels(bucket, info.Labels); err != nil {
		return err
	}

	if err := boltutil.WriteAnnotations(bucket, info.Annotations); err != nil {
		return err
	}

	if err := boltutil.WriteExtensions(bucket, info.Extensions); err != nil {
		return err
	}

	spec, err := json.Marshal(info.RuntimeSpec)
	if err != nil {
		return errors.Wrap(err, "failed to marshal runtime spec")
	}

	if err := bucket.Put([]byte("runtime_spec"), spec); err != nil {
		return err
	}

	return nil
}
