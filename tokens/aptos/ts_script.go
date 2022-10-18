package aptos

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

func GetCurrentAbPath() string {
	dir := GetCurrentAbPathByExecutable()
	tmpDir, _ := filepath.EvalSymlinks(os.TempDir())
	callerDir := GetCurrentAbPathByCaller()
	if strings.Contains(dir, tmpDir) {
		return callerDir
	}
	return dir
}

func GetCurrentAbPathByExecutable() string {
	exePath, err := os.Executable()
	if err != nil {
		log.Fatal(err)
	}
	res, _ := filepath.EvalSymlinks(filepath.Dir(exePath))
	fmt.Println("GetCurrentAbPathByExecutable", res)
	return res
}

func GetCurrentAbPathByCaller() string {
	var abPath string
	_, filename, _, ok := runtime.Caller(0)
	if ok {
		abPath = path.Dir(filename)
	}
	fmt.Println("GetCurrentAbPathByCaller", abPath)
	return abPath
}

func InstallTsModules() {
	cmd := exec.Command("bash", "-c", "yarn -i")
	cmd.Dir = GetCurrentAbPathByCaller()
	out, err := cmd.CombinedOutput()
	stats := strings.Split(string(out), "\n")
	for _, stat := range stats {
		fmt.Println("init Aptos script evn", stat)
	}
	if err != nil {
		log.Fatalf("cmd.Run() failed with %s\n", err)
	}
}

func RunTxHashScript(txbody, argTypes *string, chainId uint) (string, error) {
	cmd := exec.Command("bash", "-c", fmt.Sprintf("yarn txhash '%s' %s %d", *txbody, *argTypes, chainId))
	cmd.Dir = GetCurrentAbPathByCaller()
	out, err := cmd.CombinedOutput()
	stats := strings.Split(string(out), "\n")
	for _, stat := range stats {
		fmt.Println("CalcTxHashByTSScirpt", stat)
	}
	if err != nil {
		return "", fmt.Errorf(string(out))
	}
	if len(stats) < 3 {
		return "", fmt.Errorf("CalcTxHashByTSScirpt ts output error")
	}
	if !strings.HasPrefix(stats[len(stats)-2], "Done") {
		return "", fmt.Errorf(stats[len(stats)-2])
	}
	return stats[len(stats)-3], nil
}
