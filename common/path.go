// Copyright 2014 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package common

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/anyswap/CrossChain-Router/v3/log"
)

// MustRunBashCommand for tool usage
func MustRunBashCommand(cwd, cmdStr string) []string {
	cmd := exec.Command("bash", "-c", cmdStr)
	cmd.Dir = cwd
	output, err := cmd.CombinedOutput()
	outputStr := strings.TrimSuffix(string(output), "\n")
	if err != nil {
		log.Println(outputStr)
		log.Printf("run command %v (cwd='%v') failed. error is '%v'", cmdStr, cwd, err)
		return []string{}
	} else {
		log.Printf("run command %v (cwd='%v') success.\n%v", cmdStr, cwd, outputStr)
		return strings.Split(outputStr, "\n")
	}
}

func MustRunBashCommandWithEnv(cwd, cmdStr string, env map[string]string) []string {
	for k, v := range env {
		err := os.Setenv(k, v)
		if err != nil {
			fmt.Println(err.Error())
		}
	}
	cmd := exec.Command("bash", "-c", cmdStr)
	cmd.Dir = cwd
	output, err := cmd.CombinedOutput()
	outputStr := strings.TrimSuffix(string(output), "\n")
	if err != nil {
		log.Println(outputStr)
		log.Printf("run command %v (cwd='%v') failed. error is '%v'", cmdStr, cwd, err)
		return []string{}
	} else {
		log.Printf("run command %v (cwd='%v') success.\n%v", cmdStr, cwd, outputStr)
		return strings.Split(outputStr, "\n")
	}
}

// MakeName creates a node name that follows the ethereum convention
// for such names. It adds the operation system name and Go runtime version
// the name.
func MakeName(name, version string) string {
	return fmt.Sprintf("%s/v%s/%s/%s", name, version, runtime.GOOS, runtime.Version())
}

// FileExist checks if a file exists at filePath.
func FileExist(filePath string) bool {
	_, err := os.Stat(filePath)
	if err != nil && os.IsNotExist(err) {
		return false
	}

	return true
}

// AbsolutePath returns datadir + filename, or filename if it is absolute.
func AbsolutePath(datadir, filename string) string {
	if filepath.IsAbs(filename) {
		return filename
	}
	return filepath.Join(datadir, filename)
}

// ExecuteDir returns the execute directory
func ExecuteDir() (string, error) {
	return filepath.Abs(filepath.Dir(os.Args[0]))
}

// CurrentDir current directory
func CurrentDir() (string, error) {
	return os.Getwd()
}
