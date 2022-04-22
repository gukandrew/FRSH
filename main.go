package main

import (
	"fmt"
	"os/exec"
)

func main() {
	res, err := exec.Command("ls", "-la").Output()
	if err != nil {
		panic(err)
	}
	fmt.Printf("OUTPUT: %s", res)
}
