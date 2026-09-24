package main

import (
	"encoding/json"
	"fmt"
	"os"

	"Farm_Go/internal/runtime/qqpatch"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: install-current-qq-patch <game.js>")
		os.Exit(2)
	}
	buttonScript, err := os.ReadFile("resources/wmpf/button.js")
	if err != nil {
		panic(err)
	}
	hostScript, err := os.ReadFile("resources/qq/qq-host.js")
	if err != nil {
		panic(err)
	}
	result := qqpatch.Install(qqpatch.Options{
		TargetPath:   os.Args[1],
		ButtonScript: string(buttonScript),
		HostScript:   string(hostScript),
		HostVersion:  "farm-go-host-1",
	})
	encoded, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(encoded))
	if !result.OK {
		os.Exit(1)
	}
}
