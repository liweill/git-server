package repo

import (
	"git-server/internal/conf"
	"git-server/internal/context"
	"git-server/internal/form"
	"git-server/internal/repoutil"
	"git-server/internal/type"
	"github.com/pkg/errors"
	"os"
	"os/exec"
	"path"
	"strings"
)

func CreatePost(c *context.Context, f form.Repo) {
	repoPath := repoutil.RepoPath(f.Code, f.RepoName)
	FullRepoName := repoutil.FullRepoName(f.Code, f.RepoName)
	if !repoExists(repoPath) {
		err := initRepo(FullRepoName)
		if err != nil {
			result := _type.FaildResult(err)
			c.JSON(500, result)
			return
		}
		data := repoutil.CloneLink{
			HTTPS: repoutil.HTTPSCloneURL(f.Code, f.RepoName),
		}
		result := _type.SuccessResult(data)
		c.JSON(200, result)
		return
	}
	err := errors.New("repoPath has exists")
	result := _type.FaildResult(err)
	c.JSON(500, result)
}

func repoExists(p string) bool {
	_, err := os.Stat(path.Join(p, "objects"))
	return err == nil
}

func initRepo(FullRepoName string) error {
	fullPath := path.Join(conf.Repository.Root, FullRepoName) + ".git"
	if err := exec.Command("git", "init", "--bare", fullPath).Run(); err != nil {
		return err
	}
	return nil
}

func DeleteRepo(c *context.Context, f form.Repo) {
	repoPath := repoutil.RepoPath(f.Code, f.RepoName)
	file, err := os.Lstat(repoPath)
	if err != nil || file == nil {
		c.JSON(500, _type.FaildResult(err))
		return
	}
	err = os.RemoveAll(repoPath)
	if err != nil || file == nil {
		c.JSON(500, _type.FaildResult(err))
		return
	}
	repoName := repoutil.FullRepoName(f.Code, f.RepoName)
	c.JSON(200, _type.SuccessResult(repoName))
}

func ListRepo(c *context.Context, f form.ListRepo) {
	//groups, err := auth.GetUserGroups(f.UserId, c.Req.Header.Get("Authorization"))
	//if err != nil {
	//	result := _type.FaildResult(err)
	//	c.JSON(500, result)
	//	return
	//}
	//groups = append(groups, f.Code)
	//repos, err := getAllRepos()
	//if err != nil {
	//	result := _type.FaildResult(err)
	//	c.JSON(500, result)
	//	return
	//}
	//res := make([]string, 0)
	//for _, r := range repos {
	//	for _, g := range groups {
	//		if strings.HasPrefix(r, g) {
	//			res = append(res, r)
	//		}
	//	}
	//}
	repos, err := GetRepos(f.Code)
	if err != nil {
		result := _type.FaildResult(err)
		c.JSON(500, result)
	}
	result := _type.SuccessResult(repos)
	c.JSON(200, result)
}

func getAllRepos() ([]string, error) {
	fullPath := conf.Repository.Root
	dirs, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, err
	}
	repos := make([]string, 0)
	for _, repoDir := range dirs {
		if repoDir.IsDir() {
			subDirs, err := os.ReadDir(path.Join(fullPath, repoDir.Name()))
			if err != nil {
				return nil, err
			}
			for _, d := range subDirs {
				if d.IsDir() && strings.HasSuffix(d.Name(), ".git") {
					repos = append(repos, path.Join(repoDir.Name(), d.Name()))
				}
			}
		}
	}
	return repos, nil
}
func GetRepos(code string) ([]string, error) {
	fullPath := conf.Repository.Root
	dirs, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, err
	}
	repos := make([]string, 0)
	for _, repoDir := range dirs {
		if repoDir.Name() == code && repoDir.IsDir() {
			subDirs, err := os.ReadDir(path.Join(fullPath, repoDir.Name()))
			if err != nil {
				return nil, err
			}
			for _, d := range subDirs {
				if d.IsDir() && strings.HasSuffix(d.Name(), ".git") {
					repos = append(repos, path.Join(repoDir.Name(), d.Name()))
				}
			}
		}
	}
	return repos, nil
}
