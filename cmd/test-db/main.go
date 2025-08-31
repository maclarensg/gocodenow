package main

import (
	"fmt"
	"os"

	"gocodenow/internal/storage"
)

func main() {
	fmt.Println("🗄️ Testing gocodenow database functionality...")
	
	if err := storage.TestDatabase(); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Database test failed: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Println("🎉 All database tests passed!")
}