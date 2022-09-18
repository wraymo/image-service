// Copyright 2020 Ant Group. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package tool

import (
	"fmt"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/v1"
	"os"
	"os/exec"
	"path/filepath"
)

func run(cmd string, args ...string) error {
	_cmd := exec.Command("sh", "-c", cmd)
	_cmd.Stdout = os.Stdout
	_cmd.Stderr = os.Stderr
	return _cmd.Run()
}

func runWithOutput(cmd string, args ...string) ([]byte, error) {
	_cmd := exec.Command("sh", "-c", cmd)
	_cmd.Stderr = os.Stderr
	return _cmd.Output()
}

type Image struct {
	Source     string
	SourcePath string
	Rootfs     string
	Layers     []v1.Layer
}

// FIXME: better to use `archive.Apply` in containerd package to
// unpack image layer to overlayfs lowerdir.
func (image *Image) Pull() error {
	//return run(fmt.Sprintf("docker pull %s", image.Source))
	////ref, err := reference.ParseNormalizedNamed(image.Source)
	////if err != nil {
	////	return fmt.Errorf("error parsing additional-tag '%s': %v", image, err) // TODO: modify
	////}
	//
	//policy, err := signature.DefaultPolicy(nil)
	//if err != nil {
	//	return fmt.Errorf("error parsing additional-tag '%s': %v", image, err) // TODO: modify
	//}
	//policyContext, err := signature.NewPolicyContext(policy)
	//if err != nil {
	//	return fmt.Errorf("error parsing additional-tag '%s': %v", image, err) // TODO: modify
	//}
	//
	//srcref, err := alltransports.ParseImageName(fmt.Sprintf("docker://%s", image.Source))
	//destref, err := alltransports.ParseImageName("dir:/home/wraymo/test_image_store")
	//copy.Image(context.Background(), policyContext, srcref, destref, &copy.Options{})
	i, err := crane.Pull(image.Source)
	if err != nil {
		return fmt.Errorf("crane.Pull function failed")
	}

	//m, err := i.Manifest()
	//if err != nil {
	//	return fmt.Errorf("mkdir %s failed!\n")
	//}

	layers, err := i.Layers()
	if err != nil {
		return fmt.Errorf("mkdir %s failed!\n")
	}
	image.Layers = layers

	err = os.MkdirAll(image.SourcePath, 0755)
	if err != nil {
		return fmt.Errorf("mkdir %s failed!\n")
	}

	imagePath := filepath.Join(image.SourcePath, "image.tar")
	err = crane.Save(i, image.Source, imagePath)
	if err != nil {
		return fmt.Errorf("crane.Pull function failed")
	}

	return nil
}

// Mount parses lowerdir and upperdir options of overlayfs from
// `docker inspect` command output, and mounts rootfs of OCI image.
func (image *Image) Mount() error {
	image.Umount()

	//output, err := runWithOutput(fmt.Sprintf("docker inspect %s", image.Source))
	//if err != nil {
	//	return err
	//}
	//
	//dirs := []string{}
	//upperDir := gjson.Get(string(output), "0.GraphDriver.Data.UpperDir")
	//if !upperDir.Exists() {
	//	return errors.New("Not found upper dir in image info")
	//}
	//dirs = append(dirs, strings.Split(upperDir.String(), ":")...)
	//
	//lowerDir := gjson.Get(string(output), "0.GraphDriver.Data.LowerDir")
	//if lowerDir.Exists() {
	//	dirs = append(dirs, strings.Split(lowerDir.String(), ":")...)
	//}
	//if len(dirs) == 1 {
	//	dirs = append(dirs, image.Rootfs)
	//}
	//
	//lowerOption := strings.Join(dirs, ":")
	//
	//// Handle long options string overed 4096 chars, split them to
	//// two overlay mounts.
	//if len(lowerOption) >= 4096 {
	//	half := (len(dirs) - 1) / 2
	//	upperDirs := dirs[half+1:]
	//	lowerDirs := dirs[:half+1]
	//	lowerOverlay := image.Rootfs + "_lower"
	//	if err := os.MkdirAll(lowerOverlay, 0755); err != nil {
	//		return err
	//	}
	//	if err := run(fmt.Sprintf(
	//		"mount -t overlay overlay -o lowerdir=%s %s",
	//		strings.Join(upperDirs, ":"), lowerOverlay,
	//	)); err != nil {
	//		return err
	//	}
	//	lowerDirs = append(lowerDirs, lowerOverlay)
	//	lowerOption = strings.Join(lowerDirs, ":")
	//}

	//if err := run(fmt.Sprintf(
	//	"mount -t overlay overlay -o lowerdir=%s %s",
	//	lowerOption, image.Rootfs,
	//)); err != nil {
	//	return err
	//}

	return nil
}

// Umount umounts rootfs mountpoint of OCI image.
func (image *Image) Umount() error {
	lowerOverlay := image.Rootfs + "_lower"
	if _, err := os.Stat(lowerOverlay); err == nil {
		if err := run(fmt.Sprintf("umount %s", lowerOverlay)); err != nil {
			return err
		}
	}

	if _, err := os.Stat(image.Rootfs); err == nil {
		if err := run(fmt.Sprintf("umount %s", image.Rootfs)); err != nil {
			return err
		}
	}

	return nil
}

func (image *Image) extractLayers() error {
	//for i, l := range image.Layers {
	//	l.Compressed()
	//}
	return nil
}
