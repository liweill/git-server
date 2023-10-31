package main

import (
	"fmt"
	"git-server/internal/auth"
	"git-server/internal/conf"
	"git-server/internal/route/repo"
	"log"
	"os"
	"os/exec"
	"strings"
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
func TestBranch(t *testing.T) {
	repoPath := "C:/Users/15713/gogs-repositories/13684856438/repo.git" // 替换为实际的仓库路径

	// 设置执行命令的工作目录
	cmd := exec.Command("git", "symbolic-ref", "HEAD")
	cmd.Dir = repoPath

	// 执行命令并获取输出结果
	output, err := cmd.Output()
	if err != nil {
		log.Fatal(err)
	}

	// 输出命令输出结果
	fmt.Println(string(output))
	result := strings.Split(string(output), "/")
	r := result[len(result)-1]
	r = strings.Replace(r, "\n", "", -1)
	fmt.Println("result:", r)
}
