package main

import (
	"fmt"
	"git-server/internal/auth"
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
	fmt.Printf("%v", conf.CustomConf)
}
func TestList(t *testing.T) {
	err := conf.Init()
	if err != nil {
		log.Fatalf("init config error: %v", err)
		os.Exit(-1)
	}
	fmt.Println(repo.GetRepos("root"))
}

func TestConf(t *testing.T) {
	fmt.Printf(conf.AppPath())
}

func TestAuth(t *testing.T) {
	err := conf.Init()
	if err != nil {
		log.Fatalf("init config error: %v", err)
		os.Exit(-1)
	}
	auth.Init()
	authUser, err := auth.Authenticator.Authenticate("13684856438", "@LiWei1133")
	if err == nil && authUser.FullName != "" {
		// authorize
		if flag, _ := auth.Authorizer.Authorize(authUser, "13684856438"); !flag {
			fmt.Printf("1111")
			return
		}
	} else {
		fmt.Printf("222")
		return
	}
	fmt.Println("132")
}
