package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Println("Usage: dcsbr <source_folder> <destination_folder>")
		fmt.Println("Example: dcsbr C:/my/stack C:/my/backups")
		os.Exit(1)
	}
	srcPath := os.Args[1]
	dstPath := os.Args[2]
	fmt.Printf("Starting backup of '%s' to '%s'...\n", srcPath, dstPath)
	err := BackupComposeStack(srcPath, dstPath)
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
	fmt.Println("Backup completed successfully.")
}
