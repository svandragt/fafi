package sander

import (
	"io"
	"os"
)

func CopyToTmp(sourcePath, tmpFilename string) (**os.File, error) {
	// Open the source file for reading
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return nil, err
	}
	defer func(sourceFile *os.File) {
		err := sourceFile.Close()
		if err != nil {
			return
		}
	}(sourceFile)

	// Get the system's default temporary directory
	tmpDir := os.TempDir()

	// CreateOrGet a temporary file in the tmp directory
	tmpFile, err := os.CreateTemp(tmpDir, tmpFilename)
	if err != nil {
		return nil, err
	}
	defer func(tmpFile *os.File) {
		err := tmpFile.Close()
		if err != nil {
			return
		}
	}(tmpFile)

	// Copy the contents of the source file to the temporary file
	_, err = io.Copy(tmpFile, sourceFile)
	if err != nil {
		return nil, err
	}

	return &tmpFile, nil
}
