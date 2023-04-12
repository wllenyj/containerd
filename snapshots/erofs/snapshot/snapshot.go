//go:build linux
// +build linux

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

package erofs

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/containerd/containerd/diff/erofs"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/snapshots"
	"github.com/containerd/containerd/snapshots/erofs/pkg/label"
	"github.com/containerd/containerd/snapshots/storage"
	"github.com/pkg/errors"
)

const (
	meta          string = "meta"
	tar           string = "tar"
	BootstrapFile string = "image/image.boot"
)

type snapshotter struct {
	root string
	ms   *storage.MetaStore
}

func NewSnapshotter(root string) (snapshots.Snapshotter, error) {
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	ms, err := storage.NewMetaStore(filepath.Join(root, "metadata.db"))
	if err != nil {
		return nil, err
	}
	if err := os.Mkdir(filepath.Join(root, "snapshots"), 0700); err != nil && !os.IsExist(err) {
		return nil, err
	}

	return &snapshotter{
		root: root,
		ms:   ms,
	}, nil
}

func (o *snapshotter) Stat(ctx context.Context, key string) (snapshots.Info, error) {
	return snapshots.Info{}, nil
}

func (o *snapshotter) Update(ctx context.Context, info snapshots.Info, fieldpaths ...string) (snapshots.Info, error) {
	return snapshots.Info{}, nil
}

func (o *snapshotter) Usage(ctx context.Context, key string) (snapshots.Usage, error) {
	return snapshots.Usage{}, nil
}

func (o *snapshotter) Prepare(ctx context.Context, key, parent string, opts ...snapshots.Opt) ([]mount.Mount, error) {
	logCtx := log.G(ctx).WithField("key", key).WithField("parent", parent)
	base, s, err := o.createSnapshotOverlay(ctx, snapshots.KindActive, key, parent, opts)
	if err != nil {
		return nil, err
	}
	logCtx.Infof("prepare snapshot with labels %v", base.Labels)

	if _, ok := base.Labels[label.TargetSnapshotRef]; ok {
		logCtx.Infof("snapshot ref label found %s", label.TargetSnapshotRef)
		return o.bindmount(s), nil
	} else {
		logCtx.Infof("prepare for container layer %s", key)
		bootstrapPath := filepath.Join(o.upperPath(s.ID), BootstrapFile)
		if err = o.mergeBootstraps(ctx, s.ParentIDs, bootstrapPath); err != nil {
			return nil, err
		}
		tars, err := o.getTars(ctx, s.ParentIDs)
		if err != nil {
			return nil, err
		}
		var tarDevs []string
		for _, tarfile := range tars {
			tarDev, err := mount.AttachLoopDevice(tarfile)
			if err != nil {
				return nil, err
			}
			tarDevs = append(tarDevs, tarDev)
		}
		bootstrapDev, err := mount.AttachLoopDevice(bootstrapPath)
		if err != nil {
			return nil, err
		}
		erofsMountPoint, err := createRandomDir(o.snapshotRoot())
		if err != nil {
			return nil, err
		}
		if err = o.doErofsMount(ctx, tarDevs, bootstrapDev, erofsMountPoint); err != nil {
			return nil, err
		}
		return o.erofsmount(s, erofsMountPoint), nil
	}
}

func createRandomDir(baseDir string) (string, error) {
	rand.Seed(time.Now().UnixNano())
	dirName := fmt.Sprintf("dir-%d", rand.Intn(1000))
	if err := os.MkdirAll(filepath.Join(baseDir, dirName), 0755); err != nil {
		return "", err
	}

	return filepath.Join(baseDir, dirName), nil
}

func (o *snapshotter) doErofsMount(ctx context.Context, tarDevs []string, bootstrapDev string, mnt string) error {
	ctx, t, err := o.ms.TransactionContext(ctx, false)
	if err != nil {
		return err
	}
	defer t.Rollback()

	// mount -t erofs -o device=blob1,device=blob2,device=blob3 bootstrap mnt
	args := []string{"-t erofs -o"}

	var devs string
	for _, t := range tarDevs {
		devs += fmt.Sprintf("device=%s,", t)
	}
	devs = strings.TrimSuffix(devs, ",")

	args = append(args, devs, bootstrapDev, mnt)

	return erofs.ExecCommand(ctx, "mount", args...)
}

func (o *snapshotter) mergeBootstraps(ctx context.Context, ids []string, target string) error {
	ctx, t, err := o.ms.TransactionContext(ctx, false)
	if err != nil {
		return err
	}
	defer t.Rollback()

	bootstraps, err := o.getBootstraps(ctx, ids)
	if err != nil {
		log.G(ctx).Infof("failed to get blobs' name from snapshots key %v", err)
		return nil
	}

	path := erofs.DetectErofsUtils()
	if path == "" {
		return fmt.Errorf("failed to find erofs utils.")
	}
	args := []string{"merge"}
	args = append(args, bootstraps...)
	args = append(args, "-B", target)
	return erofs.ExecCommand(ctx, path, args...)
}

func (o *snapshotter) getChainFiles(ctx context.Context, ids []string, filename string) ([]string, error) {
	ctx, t, err := o.ms.TransactionContext(ctx, false)
	if err != nil {
		return nil, err
	}
	defer t.Rollback()

	var files []string
	for _, id := range ids {
		file := filepath.Join(o.snapshotDir(id), filename)
		if _, err := os.Stat(file); err != nil {
			return nil, errors.Wrapf(err, "failed to get %s %s", filename, file)
		}
		files = append(files, file)
	}
	return files, nil
}

func (o *snapshotter) getTars(ctx context.Context, ids []string) ([]string, error) {
	return o.getChainFiles(ctx, ids, tar)
}

func (o *snapshotter) getBootstraps(ctx context.Context, ids []string) ([]string, error) {
	return o.getChainFiles(ctx, ids, meta)
}

func (o *snapshotter) bindmount(s storage.Snapshot) []mount.Mount {
	roFlag := "rw"
	if s.Kind == snapshots.KindView {
		roFlag = "ro"
	}

	return []mount.Mount{
		{
			Source: o.upperPath(s.ID),
			Type:   "bind",
			Options: []string{
				roFlag,
				"rbind",
			},
		},
	}
}

func (o *snapshotter) erofsmount(s storage.Snapshot, erofsMountPoint string) []mount.Mount {
	var options []string
	// set index=off when mount overlayfs
	// if o.indexOff {
	// 	options = append(options, "index=off")
	// }

	// if o.userxattr {
	// 	options = append(options, "userxattr")
	// }

	if s.Kind == snapshots.KindActive {
		options = append(options,
			fmt.Sprintf("workdir=%s", o.workPath(s.ID)),
			fmt.Sprintf("upperdir=%s", o.upperPath(s.ID)),
		)
	} else if len(s.ParentIDs) == 1 {
		return []mount.Mount{
			{
				Source: o.upperPath(s.ParentIDs[0]),
				Type:   "bind",
				Options: []string{
					"ro",
					"rbind",
				},
			},
		}
	}

	parentPaths := make([]string, len(s.ParentIDs))
	for i := range s.ParentIDs {
		parentPaths[i] = o.upperPath(s.ParentIDs[i])
	}

	options = append(options, fmt.Sprintf("lowerdir=%s", erofsMountPoint))
	return []mount.Mount{
		{
			Type:    "overlay",
			Source:  "overlay",
			Options: options,
		},
	}
}

func (o *snapshotter) View(ctx context.Context, key, parent string, opts ...snapshots.Opt) ([]mount.Mount, error) {
	return []mount.Mount{
		{
			Source: "",
			Type:   "erofs",
		},
	}, nil
}

func (o *snapshotter) Mounts(ctx context.Context, key string) ([]mount.Mount, error) {
	return []mount.Mount{
		{
			Source: "",
			Type:   "erofs",
		},
	}, nil
}

func (o *snapshotter) Commit(ctx context.Context, name, key string, opts ...snapshots.Opt) error {
	return nil
}

func (o *snapshotter) Remove(ctx context.Context, key string) (err error) {
	return nil
}

func (o *snapshotter) Walk(ctx context.Context, fn snapshots.WalkFunc, fs ...string) error {
	return nil
}

func (o *snapshotter) Close() error {
	return nil
}

func (o *snapshotter) createSnapshotNydus(ctx context.Context, kind snapshots.Kind, key, parent string, opts []snapshots.Opt) (info *snapshots.Info, _ storage.Snapshot, err error) {
	ctx, t, err := o.ms.TransactionContext(ctx, true)
	if err != nil {
		return nil, storage.Snapshot{}, err
	}
	rollback := true
	defer func() {
		if rollback {
			if rerr := t.Rollback(); rerr != nil {
				log.G(ctx).WithError(rerr).Warn("failed to rollback transaction")
			}
		}
	}()

	var base snapshots.Info
	for _, opt := range opts {
		if err := opt(&base); err != nil {
			return &base, storage.Snapshot{}, err
		}
	}

	var td, path string
	defer func() {
		if td != "" {
			if err1 := o.cleanupSnapshotDirectory(ctx, td); err1 != nil {
				log.G(ctx).WithError(err1).Warn("failed to cleanup temp snapshot directory")
			}
		}
		if path != "" {
			if err1 := o.cleanupSnapshotDirectory(ctx, path); err1 != nil {
				log.G(ctx).WithError(err1).WithField("path", path).Error("failed to reclaim snapshot directory, directory may need removal")
				err = errors.Wrapf(err, "failed to remove path: %v", err1)
			}
		}
	}()

	td, err = o.prepareDirectory(ctx, o.snapshotRoot(), kind)
	if err != nil {
		return nil, storage.Snapshot{}, errors.Wrap(err, "failed to create prepare snapshot dir")
	}

	s, err := storage.CreateSnapshot(ctx, kind, key, parent, opts...)
	if err != nil {
		return nil, storage.Snapshot{}, errors.Wrap(err, "failed to create snapshot")
	}

	if len(s.ParentIDs) > 0 {
		st, err := os.Stat(o.upperPath(s.ParentIDs[0]))
		if err != nil {
			return nil, storage.Snapshot{}, errors.Wrap(err, "failed to stat parent")
		}

		// FIXME: Why only change owner of having parent?
		if err := lchown(filepath.Join(td, "fs"), st); err != nil {
			return nil, storage.Snapshot{}, errors.Wrap(err, "failed to chown")
		}
	}

	path = o.snapshotDir(s.ID)
	if err = os.Rename(td, path); err != nil {
		return nil, storage.Snapshot{}, errors.Wrap(err, "failed to rename")
	}
	td = ""

	rollback = false
	if err = t.Commit(); err != nil {
		return nil, storage.Snapshot{}, errors.Wrap(err, "commit failed")
	}
	path = ""

	return &base, s, nil
}

func (o *snapshotter) createSnapshotOverlay(ctx context.Context, kind snapshots.Kind, key, parent string, opts []snapshots.Opt) (info *snapshots.Info, s storage.Snapshot, err error) {
	var td, path string

	var base snapshots.Info
	for _, opt := range opts {
		if err := opt(&base); err != nil {
			return &base, storage.Snapshot{}, err
		}
	}

	defer func() {
		if err != nil {
			if td != "" {
				if err1 := os.RemoveAll(td); err1 != nil {
					log.G(ctx).WithError(err1).Warn("failed to cleanup temp snapshot directory")
				}
			}
			if path != "" {
				if err1 := os.RemoveAll(path); err1 != nil {
					log.G(ctx).WithError(err1).WithField("path", path).Error("failed to reclaim snapshot directory, directory may need removal")
					err = fmt.Errorf("failed to remove path: %v: %w", err1, err)
				}
			}
		}
	}()

	if err := o.ms.WithTransaction(ctx, true, func(ctx context.Context) (err error) {
		snapshotDir := filepath.Join(o.root, "snapshots")
		td, err = o.prepareDirectory(ctx, snapshotDir, kind)
		if err != nil {
			return fmt.Errorf("failed to create prepare snapshot dir: %w", err)
		}

		s, err = storage.CreateSnapshot(ctx, kind, key, parent, opts...)
		if err != nil {
			return fmt.Errorf("failed to create snapshot: %w", err)
		}

		if len(s.ParentIDs) > 0 {
			st, err := os.Stat(o.upperPath(s.ParentIDs[0]))
			if err != nil {
				return fmt.Errorf("failed to stat parent: %w", err)
			}

			stat := st.Sys().(*syscall.Stat_t)
			if err := os.Lchown(filepath.Join(td, "fs"), int(stat.Uid), int(stat.Gid)); err != nil {
				return fmt.Errorf("failed to chown: %w", err)
			}
		}

		path = filepath.Join(snapshotDir, s.ID)
		if err = os.Rename(td, path); err != nil {
			return fmt.Errorf("failed to rename: %w", err)
		}
		td = ""

		return nil
	}); err != nil {
		return nil, storage.Snapshot{}, err
	}

	return &base, s, nil
}

func (o *snapshotter) upperPath(id string) string {
	return filepath.Join(o.root, "snapshots", id, "fs")
}

func (o *snapshotter) workPath(id string) string {
	return filepath.Join(o.root, "snapshots", id, "work")
}

func (o *snapshotter) snapshotRoot() string {
	return filepath.Join(o.root, "snapshots")
}

func (o *snapshotter) snapshotDir(id string) string {
	return filepath.Join(o.snapshotRoot(), id)
}

func (o *snapshotter) prepareDirectory(ctx context.Context, snapshotDir string, kind snapshots.Kind) (string, error) {
	td, err := ioutil.TempDir(snapshotDir, "new-")
	if err != nil {
		return "", errors.Wrap(err, "failed to create temp dir")
	}

	if err := os.Mkdir(filepath.Join(td, "fs"), 0755); err != nil {
		return td, err
	}

	if kind == snapshots.KindActive {
		if err := os.Mkdir(filepath.Join(td, "work"), 0711); err != nil {
			return td, err
		}
	}

	return td, nil
}

func (o *snapshotter) cleanupSnapshotDirectory(ctx context.Context, dir string) error {
	// On a remote snapshot, the layer is mounted on the "fs" directory.
	// We use Filesystem's Unmount API so that it can do necessary finalization
	// before/after the unmount.
	log.G(ctx).WithField("dir", dir).Infof("cleanupSnapshotDirectory %s", dir)

	if err := os.RemoveAll(dir); err != nil {
		return errors.Wrapf(err, "failed to remove directory %q", dir)
	}
	return nil
}
