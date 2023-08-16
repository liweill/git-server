package conf

import (
	"fmt"
	"log"
	"os"
	"testing"
)

func TestName(t *testing.T) {
	err := Init()
	if err != nil {
		log.Fatalf("init config error: %v", err)
		os.Exit(-1)
	}
	fmt.Println(Auth)
}
