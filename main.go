/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package main

import (
	"fmt"
	"os"

	"github.com/aiLeonardo/cryptotips/cmd"
	"github.com/aiLeonardo/cryptotips/lib"
)

func main() {
	defer lib.RecoverInfo()
	lib.LoadConfig()
	fmt.Printf("main args: %v\n\n", os.Args)

	cmd.Execute()
}
