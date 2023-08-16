package repoutil

import (
	"fmt"
	"git-server/internal/conf"
	"path/filepath"
	"strings"
)

type CloneLink struct {
	SSH   string
	HTTPS string
}

func UserPath(user string) string {
	return filepath.Join(conf.Repository.Root, strings.ToLower(user))
}

func RepoPath(userName, repoName string) string {
	return filepath.Join(UserPath(userName), strings.ToLower(repoName)+".git")
}

func FullRepoName(userName, repoName string) string {
	return fmt.Sprintf("%s/%s", userName, repoName)
}
func HTTPSCloneURL(owner, repo string) string {
	return fmt.Sprintf("%s%s/%s.git", conf.Server.ExternalURL, owner, repo)
}
