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
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/containerd/containerd/diff/erofs"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/snapshots"
	"github.com/containerd/containerd/snapshots/erofs/pkg/label"
	"github.com/containerd/containerd/snapshots/storage"
	"github.com/containerd/continuity/fs"
	"github.com/pkg/errors"
)

const (
	meta          string = "metadata"
	tar           string = "tar"
	BootstrapFile string = "image.boot"
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

func (o *snapshotter) Stat(ctx context.Context, key string) (info snapshots.Info, err error) {
	if err := o.ms.WithTransaction(ctx, false, func(ctx context.Context) error {
		_, info, _, err = storage.GetInfo(ctx, key)
		return err
	}); err != nil {
		return info, err
	}

	return info, nil
}

func (o *snapshotter) Update(ctx context.Context, info snapshots.Info, fieldpaths ...string) (snapshots.Info, error) {
	return snapshots.Info{}, nil
}

func (o *snapshotter) Usage(ctx context.Context, key string) (snapshots.Usage, error) {
	return snapshots.Usage{}, nil
}

func (o *snapshotter) Prepare(ctx context.Context, key, parent string, opts ...snapshots.Opt) ([]mount.Mount, error) {
	return o.createSnapshot(ctx, snapshots.KindActive, key, parent, opts)
}

func (o *snapshotter) View(ctx context.Context, key, parent string, opts ...snapshots.Opt) ([]mount.Mount, error) {
	return o.createSnapshot(ctx, snapshots.KindView, key, parent, opts)
}

func (o *snapshotter) createSnapshot(ctx context.Context, kind snapshots.Kind, key, parent string, opts []snapshots.Opt) ([]mount.Mount, error) {
	logCtx := log.G(ctx).WithField("key", key).WithField("parent", parent)
	base, s, err := o.createSnapshotOverlay(ctx, kind, key, parent, opts)
	if err != nil {
		return nil, err
	}
	logCtx.Infof("prepare snapshot with labels %v", base.Labels)

	if v, ok := base.Labels[label.TargetSnapshotRef]; ok {
		logCtx.Infof("snapshot ref label found %s:%s", label.TargetSnapshotRef, v)
		return o.bindmount(s), nil
	} else {
		logCtx.Infof("prepare for container layer %s, ids: %v", key, s.ParentIDs)
		bootstrapPath := filepath.Join(o.metaPath(s.ID), BootstrapFile)
		if err = o.mergeBootstraps(ctx, s.ParentIDs, bootstrapPath); err != nil {
			return nil, err
		}
		tars, err := o.getTars(ctx, s.ParentIDs)
		if err != nil {
			return nil, err
		}
		logCtx.Infof("prepare tar files %v", tars)
		var tarDevs []string
		for _, tarfile := range tars {
			tarDev, err := mount.AttachLoopDevice(tarfile)
			if err != nil {
				return nil, err
			}
			tarDevs = append(tarDevs, tarDev)
		}
		logCtx.Infof("prepare tar dev %v", tarDevs)
		bootstrapDev, err := mount.AttachLoopDevice(bootstrapPath)
		if err != nil {
			return nil, err
		}
		logCtx.Infof("prepare bootstrap %s to dev %v", bootstrapPath, bootstrapDev)
		//erofsMountPoint, err := createRandomDir(o.snapshotRoot())
		//if err != nil {
		//	return nil, err
		//}
		//erofsMountPoint := o.upperPath(s.ID)
		if err = o.doErofsMount(ctx, s, tarDevs, bootstrapDev); err != nil {
			return nil, err
		}
		return o.erofsMount(s), nil
	}
}

func (o *snapshotter) doErofsMount(ctx context.Context, s storage.Snapshot, tarDevs []string, bootstrapDev string) error {
	//ctx, t, err := o.ms.TransactionContext(ctx, false)
	//if err != nil {
	//	return err
	//}
	//defer t.Rollback()

	// mount -t erofs -o device=blob1,device=blob2,device=blob3 bootstrap mnt
	args := []string{"-t", "erofs", "-o"}

	var devs string
	for _, t := range tarDevs {
		devs += fmt.Sprintf("device=%s,", t)
	}
	devs = strings.TrimSuffix(devs, ",")

	args = append(args, devs, bootstrapDev, o.erofsPath(s.ID))
	log.G(ctx).Infof("prepare mount args: %#v", args)

	//return erofs.ExecCommand(ctx, "mount", args...)
	mnt := mount.Mount{
		Type: "erofs",
		Options: []string{
			devs,
		},
		Source: bootstrapDev,
	}
	if err := mnt.Mount(o.erofsPath(s.ID)); err != nil {
		log.G(ctx).Errorf("erofs mount bootstrap failed. %#v", err)
		return err
	}
	return nil
}

func (o *snapshotter) mergeBootstraps(ctx context.Context, ids []string, target string) error {
	//ctx, t, err := o.ms.TransactionContext(ctx, false)
	//if err != nil {
	//	return err
	//}
	//defer t.Rollback()

	// TODO
	//if len(ids) > 1 {
	//}

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
	log.G(ctx).Infof("merge bootstraps args: %#v", args)
	return erofs.ExecCommand(ctx, path, args...)
}

func (o *snapshotter) getChainFiles(ctx context.Context, ids []string, filename string) ([]string, error) {
	//ctx, t, err := o.ms.TransactionContext(ctx, false)
	//if err != nil {
	//	return nil, err
	//}
	//defer t.Rollback()

	var files []string
	for _, id := range ids {
		file := filepath.Join(o.metaPath(id), filename)
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
			Source: o.metaPath(s.ID),
			Type:   "bind",
			Options: []string{
				roFlag,
				"rbind",
				"erofs",
			},
		},
	}
}

func (o *snapshotter) erofsMount(s storage.Snapshot) []mount.Mount {
	var options []string
	// set index=off when mount overlayfs
	// if o.indexOff {
	// 	options = append(options, "index=off")
	// }

	// if o.userxattr {
	// 	options = append(options, "userxattr")
	// }

	//if s.Kind == snapshots.KindActive {
	options = append(options,
		fmt.Sprintf("workdir=%s", o.workPath(s.ID)),
		fmt.Sprintf("upperdir=%s", o.upperPath(s.ID)),
	)
	//}

	options = append(options, fmt.Sprintf("lowerdir=%s", o.erofsPath(s.ID)))
	return []mount.Mount{
		{
			Type:    "overlay",
			Source:  "overlay",
			Options: options,
		},
	}
}

func (o *snapshotter) Mounts(ctx context.Context, key string) (_ []mount.Mount, err error) {
	var s storage.Snapshot
	if err := o.ms.WithTransaction(ctx, false, func(ctx context.Context) error {
		s, err = storage.GetSnapshot(ctx, key)
		if err != nil {
			return fmt.Errorf("failed to get active mount: %w", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return o.erofsMount(s), nil
}

func (o *snapshotter) Commit(ctx context.Context, name, key string, opts ...snapshots.Opt) error {
	return o.ms.WithTransaction(ctx, true, func(ctx context.Context) error {
		// grab the existing id
		id, _, _, err := storage.GetInfo(ctx, key)
		if err != nil {
			return err
		}

		usage, err := fs.DiskUsage(ctx, o.metaPath(id))
		if err != nil {
			return err
		}

		if _, err = storage.CommitActive(ctx, key, name, snapshots.Usage(usage), opts...); err != nil {
			return fmt.Errorf("failed to commit snapshot %s: %w", key, err)
		}
		return nil
	})
}

func (o *snapshotter) Remove(ctx context.Context, key string) (err error) {
	var removals []string
	// Remove directories after the transaction is closed, failures must not
	// return error since the transaction is committed with the removal
	// key no longer available.
	defer func() {
		if err == nil {
			for _, dir := range removals {
				if err := os.RemoveAll(dir); err != nil {
					log.G(ctx).WithError(err).WithField("path", dir).Warn("failed to remove directory")
				}
			}
		}
	}()
	return o.ms.WithTransaction(ctx, true, func(ctx context.Context) error {
		_, _, err = storage.Remove(ctx, key)
		if err != nil {
			return fmt.Errorf("failed to remove snapshot %s: %w", key, err)
		}

		//if !o.asyncRemove {
		removals, err = o.getCleanupDirectories(ctx)
		if err != nil {
			return fmt.Errorf("unable to get directories for removal: %w", err)
		}
		//}
		return nil
	})
}

func (o *snapshotter) Walk(ctx context.Context, fn snapshots.WalkFunc, fs ...string) error {
	return nil
}

func (o *snapshotter) Close() error {
	return nil
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
			st, err := os.Stat(o.metaPath(s.ParentIDs[0]))
			if err != nil {
				return fmt.Errorf("failed to stat parent: %w", err)
			}

			stat := st.Sys().(*syscall.Stat_t)
			if err := os.Lchown(filepath.Join(td, "meta"), int(stat.Uid), int(stat.Gid)); err != nil {
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

func (o *snapshotter) erofsPath(id string) string {
	return filepath.Join(o.root, "snapshots", id, "erofs")
}

func (o *snapshotter) metaPath(id string) string {
	return filepath.Join(o.root, "snapshots", id, "meta")
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

	if err := os.Mkdir(filepath.Join(td, "meta"), 0755); err != nil {
		return td, err
	}

	if err := os.Mkdir(filepath.Join(td, "fs"), 0755); err != nil {
		return td, err
	}

	if err := os.Mkdir(filepath.Join(td, "erofs"), 0755); err != nil {
		return td, err
	}

	if kind == snapshots.KindActive {
		if err := os.Mkdir(filepath.Join(td, "work"), 0711); err != nil {
			return td, err
		}
	}

	return td, nil
}

func (o *snapshotter) getCleanupDirectories(ctx context.Context) ([]string, error) {
	ids, err := storage.IDMap(ctx)
	if err != nil {
		return nil, err
	}

	snapshotDir := filepath.Join(o.root, "snapshots")
	fd, err := os.Open(snapshotDir)
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	dirs, err := fd.Readdirnames(0)
	if err != nil {
		return nil, err
	}

	cleanup := []string{}
	for _, d := range dirs {
		if _, ok := ids[d]; ok {
			continue
		}
		cleanup = append(cleanup, filepath.Join(snapshotDir, d))
	}

	return cleanup, nil
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
