package repo

import (
	"fmt"
	"git-server/internal/conf"
	"log"
	"os"
	"testing"
)

func TestName(t *testing.T) {
	//repoExists("C:/Users/15713/gogs-repositories/13684856438/test3.git")
	err := initRepo("/13684856438/test4.git")
	if err != nil {
		fmt.Println(err)
	}
}

func TestAuth(t *testing.T) {
	err := conf.Init()
	if err != nil {
		log.Fatalf("init config error: %v", err)
		os.Exit(-1)
	}

}
