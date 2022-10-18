package main

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
)

func main() {
	install()
	run()
}

func run() {
	cmd := exec.Command("bash", "-c", "yarn txhash 11 222 333")
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("combined out:\n%s\n", string(out))
		log.Fatalf("cmd.Run() failed with %s\n", err)
	}
	stats := strings.Split(string(out), "\n")
	for i, stat := range stats {
		fmt.Printf("%d %s \n", i, stat)
	}
	fmt.Printf("txhash %s \n", stats[len(stats)-3])
}

func install() {
	cmd := exec.Command("bash", "-c", "yarn -i")
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("combined out:\n%s\n", string(out))
		log.Fatalf("cmd.Run() failed with %s\n", err)
	}
	stats := strings.Split(string(out), "\n")
	for i, stat := range stats {
		fmt.Printf("%d %s \n", i, stat)
	}
}
