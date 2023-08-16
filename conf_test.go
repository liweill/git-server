package main

import (
	"fmt"
	"git-server/internal/conf"
	"git-server/internal/route/repo"
	"log"
	"os"
	"testing"
)

func TestName(t *testing.T) {
	err := conf.Init()
	if err != nil {
		log.Fatalf("init config error: %v", err)
		os.Exit(-1)
	}
	fmt.Println(conf.Server)
}
func TestList(t *testing.T) {
	err := conf.Init()
	if err != nil {
		log.Fatalf("init config error: %v", err)
		os.Exit(-1)
	}
	fmt.Println(repo.GetRepos("root"))
}
