package sander

import (
	"io"
	"io/ioutil"
	"os"
)

func CopyToTmp(sourcePath, tmpFilename string) (**os.File, error) {
	// Open the source file for reading
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return nil, err
	}
	defer sourceFile.Close()

	// Get the system's default temporary directory
	tmpDir := os.TempDir()

	// CreateOrGet a temporary file in the tmp directory
	tmpFile, err := ioutil.TempFile(tmpDir, tmpFilename)
	if err != nil {
		return nil, err
	}
	defer tmpFile.Close()

	// Copy the contents of the source file to the temporary file
	_, err = io.Copy(tmpFile, sourceFile)
	if err != nil {
		return nil, err
	}

	return &tmpFile, nil
}
