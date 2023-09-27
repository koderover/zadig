package util

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func DeleteSubdirectories(rootDir string) error {
	// get all subdirectories under the root directory
	subDirs, err := getSubdirectories(rootDir)
	if err != nil {
		return fmt.Errorf("Error getting subdirectories: %v", err)
	}

	// delte all sub dir
	for _, subDir := range subDirs {
		err := os.RemoveAll(subDir)
		if err != nil {
			return fmt.Errorf("Error deleting %s: %v", subDir, err)
		} else {
			fmt.Printf("Deleted %s\n", subDir)
		}
	}
	return nil
}

// Get all subdirectories under the specified directory
func getSubdirectories(rootDir string) ([]string, error) {
	var subDirs []string

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && path != rootDir {
			subDirs = append(subDirs, path)
		}
		return nil
	})

	return subDirs, err
}

func DeletePathToRoot(rootDir, subDir string) error {
	// Ensure that the subdirectory is a subdirectory of the root directory
	if !isSubdirectory(rootDir, subDir) {
		return fmt.Errorf("The subdirectory is not a subdirectory of the root directory.")
	}

	// Delete recursively starting from the subdirectory and moving up to the root directory
	for subDir != rootDir {
		err := os.RemoveAll(subDir)
		if err != nil {
			return fmt.Errorf("Error deleting %s: %v\n", subDir, err)
		}
		subDir = filepath.Dir(subDir)
	}
	return nil
}

// Check if a directory is a subdirectory of another directory
func isSubdirectory(rootDir, subDir string) bool {
	rel, err := filepath.Rel(rootDir, subDir)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, "..") && !strings.HasPrefix(rel, ".")
}
