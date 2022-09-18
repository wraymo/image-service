// Copyright 2020 Ant Group. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package rule

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/dragonflyoss/image-service/contrib/nydusify/pkg/parser"
	"github.com/dragonflyoss/image-service/contrib/nydusify/pkg/remote"
	"os"
	"path/filepath"
	"reflect"
	"syscall"

	dockerconfig "github.com/docker/cli/cli/config"
	"github.com/docker/distribution/reference"

	"github.com/dragonflyoss/image-service/contrib/nydusify/pkg/checker/tool"
	"github.com/dragonflyoss/image-service/contrib/nydusify/pkg/utils"
	"github.com/pkg/errors"
	"github.com/pkg/xattr"
	"github.com/sirupsen/logrus"
)

// FilesystemRule compares file metadata and data in the two mountpoints:
// Mounted by Nydusd for Nydus image,
// Mounted by Overlayfs for OCI image.
type FilesystemRule struct {
	NydusdConfig    tool.NydusdConfig
	Source          string
	SourceMountPath string
	SourceParsed    *parser.Parsed
	SourceRemote    *remote.Remote
	Target          string
	TargetInsecure  bool
}

// Node records file metadata and file data hash.
type Node struct {
	Path    string
	Size    int64
	Mode    os.FileMode
	Rdev    uint64
	Symlink string
	UID     uint32
	GID     uint32
	Mtime   syscall.Timespec
	Xattrs  map[string][]byte
	Hash    []byte
}

type RegistryBackendConfig struct {
	Scheme string `json:"scheme"`
	Host   string `json:"host"`
	Repo   string `json:"repo"`
	Auth   string `json:"auth,omitempty"`
}

func (node *Node) String() string {
	return fmt.Sprintf(
		"Path: %s, Size: %d, Mode: %d, Rdev: %d, Symink: %s, UID: %d, GID: %d, Mtime.Sec: %d, "+
			"Mtime.Nsec: %d, Xattrs: %v, Hash: %s", node.Path, node.Size, node.Mode, node.Rdev, node.Symlink,
		node.UID, node.GID, node.Mtime.Sec, node.Mtime.Nsec, node.Xattrs, hex.EncodeToString(node.Hash),
	)
}

func (rule *FilesystemRule) Name() string {
	return "Filesystem"
}

func getXattrs(path string) (map[string][]byte, error) {
	xattrs := make(map[string][]byte)

	names, err := xattr.LList(path)
	if err != nil {
		return nil, err
	}

	for _, name := range names {
		data, err := xattr.LGet(path, name)
		if err != nil {
			return nil, err
		}
		xattrs[name] = data
	}

	return xattrs, nil
}

func (rule *FilesystemRule) walk(rootfs string) (map[string]Node, error) {
	nodes := map[string]Node{}

	if err := filepath.Walk(rootfs, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			logrus.Warnf("Failed to stat in mountpoint: %s", err)
			return nil
		}

		rootfsPath, err := filepath.Rel(rootfs, path)
		if err != nil {
			return err
		}
		rootfsPath = filepath.Join("/", rootfsPath)

		var size int64
		if !info.IsDir() {
			// Ignore directory size check
			size = info.Size()
		}

		mode := info.Mode()
		var symlink string
		if mode&os.ModeSymlink == os.ModeSymlink {
			if symlink, err = os.Readlink(path); err != nil {
				return errors.Wrapf(err, "read link %s", path)
			}
		} else {
			symlink = rootfsPath
		}

		var stat syscall.Stat_t
		if err := syscall.Lstat(path, &stat); err != nil {
			return errors.Wrapf(err, "lstat %s", path)
		}

		xattrs, err := getXattrs(path)
		if err != nil {
			logrus.Warnf("Failed to get xattr: %s", err)
		}

		// Calculate file data hash if the `backend-type` option be specified,
		// this will cause that nydusd read data from backend, it's network load
		var hash []byte
		if rule.NydusdConfig.BackendType != "" && info.Mode().IsRegular() {
			hash, err = utils.HashFile(path)
			if err != nil {
				return err
			}
		}

		node := Node{
			Path:    rootfsPath,
			Size:    size,
			Mode:    mode,
			Rdev:    stat.Rdev,
			Symlink: symlink,
			UID:     stat.Uid,
			GID:     stat.Gid,
			Mtime:   stat.Mtim,
			Xattrs:  xattrs,
			Hash:    hash,
		}
		nodes[rootfsPath] = node

		return nil
	}); err != nil {
		return nil, err
	}

	return nodes, nil
}

func (rule *FilesystemRule) mountSourceImage() (*tool.Image, error) {
	logrus.Infof("Mounting source image to %s", rule.SourceMountPath)

	if err := os.MkdirAll(rule.SourceMountPath, 0755); err != nil {
		return nil, errors.Wrap(err, "create mountpoint directory of source image")
	}

	//image := &tool.Image{
	//	Source:     rule.Source,
	//	SourcePath: rule.SourcePath,
	//	Rootfs:     rule.SourceMountPath,
	//}
	//if err := image.Pull(); err != nil {
	//	return nil, errors.Wrap(err, "pull source image")
	//}
	//if err := image.Mount(); err != nil {
	//	return nil, errors.Wrap(err, "mount source image")
	//}

	layers := rule.SourceParsed.OCIImage.Manifest.Layers
	for _, l := range layers {
		reader, err := rule.SourceRemote.Pull(context.Background(), l, true)
		if err != nil {
			return nil, errors.Wrap(err, "pull image layers from the remote registry")
		}

		if err = utils.UnpackTargz(context.Background(), rule.SourceMountPath, reader); err != nil {
			return nil, errors.Wrap(err, "unpack image layers")
		}
	}

	return nil, nil
}

func (rule *FilesystemRule) mountNydusImage() (*tool.Nydusd, error) {
	logrus.Infof("Mounting Nydus image to %s", rule.NydusdConfig.MountPath)

	if err := os.MkdirAll(rule.NydusdConfig.BlobCacheDir, 0755); err != nil {
		return nil, errors.Wrap(err, "create blob cache directory for Nydusd")
	}

	if err := os.MkdirAll(rule.NydusdConfig.MountPath, 0755); err != nil {
		return nil, errors.Wrap(err, "create mountpoint directory of Nydus image")
	}

	parsed, err := reference.ParseNormalizedNamed(rule.Target)
	if err != nil {
		return nil, err
	}

	host := reference.Domain(parsed)
	repo := reference.Path(parsed)
	if rule.NydusdConfig.BackendType == "" {
		rule.NydusdConfig.BackendType = "registry"

		if rule.NydusdConfig.BackendConfig == "" {
			config := dockerconfig.LoadDefaultConfigFile(os.Stderr)
			authConfig, err := config.GetAuthConfig(host)
			if err != nil {
				return nil, errors.Wrap(err, "get docker registry auth config")
			}

			var auth, scheme string
			if authConfig.Username != "" && authConfig.Password != "" {
				auth = base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", authConfig.Username, authConfig.Password)))
			}
			if rule.TargetInsecure {
				scheme = "http"
			} else {
				scheme = "https"
			}

			backendConfig := RegistryBackendConfig{scheme, host, repo, auth}
			bytes, err := json.Marshal(backendConfig)
			if err != nil {
				return nil, errors.Wrap(err, "parse registry backend config")
			}
			rule.NydusdConfig.BackendConfig = string(bytes)
		}
	}

	nydusd, err := tool.NewNydusd(rule.NydusdConfig)
	if err != nil {
		return nil, errors.Wrap(err, "create Nydusd daemon")
	}

	if err := nydusd.Mount(); err != nil {
		return nil, errors.Wrap(err, "mount Nydus image")
	}

	return nydusd, nil
}

func (rule *FilesystemRule) verify() error {
	logrus.Infof("Verifying filesystem for source and Nydus image")

	validate := true
	sourceNodes := map[string]Node{}

	// Concurrently walk the rootfs directory of source and Nydus image
	walkErr := make(chan error)
	go func() {
		var err error
		sourceNodes, err = rule.walk(rule.SourceMountPath)
		walkErr <- err
	}()

	nydusNodes, err := rule.walk(rule.NydusdConfig.MountPath)
	if err != nil {
		return errors.Wrap(err, "walk rootfs of Nydus image")
	}

	if err := <-walkErr; err != nil {
		return errors.Wrap(err, "walk rootfs of source image")
	}

	for path, sourceNode := range sourceNodes {
		nydusNode, exist := nydusNodes[path]
		if !exist {
			logrus.Warnf("File not found in Nydus image: %s", path)
			validate = false
			continue
		}
		delete(nydusNodes, path)

		if path != "/" && !reflect.DeepEqual(sourceNode, nydusNode) {
			logrus.Warnf("File not match in Nydus image: %s <=> %s", sourceNode.String(), nydusNode.String())
			validate = false
		}
	}

	for path := range nydusNodes {
		logrus.Warnf("File not found in source image: %s", path)
		validate = false
	}

	if !validate {
		return errors.Errorf("Failed to verify source image and Nydus image")
	}

	return nil
}

func (rule *FilesystemRule) Validate() error {
	// Skip filesystem validation if no source image be specified
	if rule.Source == "" {
		return nil
	}

	_, err := rule.mountSourceImage()
	if err != nil {
		return err
	}
	//defer image.Umount()

	nydusd, err := rule.mountNydusImage()
	if err != nil {
		return err
	}
	defer nydusd.Umount(false)

	if err := rule.verify(); err != nil {
		return err
	}

	return nil
}
