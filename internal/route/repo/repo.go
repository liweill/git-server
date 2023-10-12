package repo

import (
	"fmt"
	"git-server/internal/conf"
	"git-server/internal/context"
	"git-server/internal/form"
	"git-server/internal/repoutil"
	"git-server/internal/type"
	"github.com/gogs/git-module"
	"github.com/pkg/errors"
	"github.com/unknwon/com"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"
)

type NewBranchOpts struct {
	OldBranch string
	NewBranch string
	RepoLink  string
}
type Branch struct {
	RepoPath string
	Name     string

	IsProtected bool
	Commit      *git.Commit
}

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
	} else if err = createDelegateHooks(fullPath); err != nil {
		return fmt.Errorf("createDelegateHooks: %v", err)
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

func CreateBranch(c *context.Context, f form.CreateBranch) {
	oldBranchName := c.Repo.BranchName
	if _, err := GetBranch(f.BranchName, repoPath(c.Repo.RepoLink)); err == nil {
		c.JSON(500, _type.FaildResult(errors.New("repo.editor.branch_already_exists")))
		return
	}
	err := NewBranch(NewBranchOpts{
		OldBranch: oldBranchName,
		NewBranch: f.BranchName,
		RepoLink:  c.Repo.RepoLink,
	})
	if err != nil {
		c.JSON(500, _type.FaildResult(err))
	}
	c.JSON(200, _type.SuccessResult("create branch success."))
}
func NewBranch(opts NewBranchOpts) (err error) {
	repoWorkingPool.CheckIn(com.ToStr(opts.RepoLink))
	defer repoWorkingPool.CheckOut(com.ToStr(opts.RepoLink))

	if err := DiscardLocalRepoBranchChanges(opts.RepoLink, opts.OldBranch); err != nil {
		return fmt.Errorf("discard local repo branch[%s] changes: %v", opts.OldBranch, err)
	} else if err = UpdateLocalCopyBranch(opts.RepoLink, opts.OldBranch); err != nil {
		return fmt.Errorf("update local copy branch[%s]: %v", opts.OldBranch, err)
	}

	repoPath := repoPath(opts.RepoLink)
	localPath := LocalCopyPath(opts.RepoLink)
	fmt.Println("opts.OldBranch", opts.OldBranch, opts.NewBranch)
	if opts.OldBranch != opts.NewBranch {
		// Directly return error if new branch already exists in the server
		if git.RepoHasBranch(repoPath, opts.NewBranch) {
			return errors.New("BranchAlreadyExists!")
		}

		// Otherwise, delete branch from local copy in case out of sync
		if git.RepoHasBranch(localPath, opts.NewBranch) {
			if err = git.DeleteBranch(localPath, opts.NewBranch, git.DeleteBranchOptions{
				Force: true,
			}); err != nil {
				return fmt.Errorf("delete branch %q: %v", opts.NewBranch, err)
			}
		}

		if err := CheckoutNewBranch(opts.OldBranch, opts.NewBranch, localPath); err != nil {
			return fmt.Errorf("checkout new branch[%s] from old branch[%s]: %v", opts.NewBranch, opts.OldBranch, err)
		}
	}
	err = git.Push(localPath, "origin", opts.NewBranch)
	if err != nil {
		return fmt.Errorf("git push origin %s: %v", opts.NewBranch, err)
	}
	return nil
}

func CheckoutNewBranch(oldBranch, newBranch, localPath string) error {
	if err := git.Checkout(localPath, newBranch, git.CheckoutOptions{
		BaseBranch: oldBranch,
		Timeout:    time.Duration(300) * time.Second,
	}); err != nil {
		return fmt.Errorf("checkout [base: %s, new: %s]: %v", oldBranch, newBranch, err)
	}
	return nil
}

func GetBranch(name, repoPath string) (*Branch, error) {
	if !git.RepoHasBranch(repoPath, name) {
		return nil, errors.Errorf("branch %s does not exist", name)
	}
	return &Branch{
		RepoPath: repoPath,
		Name:     name,
	}, nil
}

var hooksTpls = map[git.HookName]string{
	"pre-receive":  "#!/usr/bin/env %s\n\"%s\" hook --branch='%s' pre-receive\n",
	"update":       "#!/usr/bin/env %s\n\"%s\" hook --config='%s' update $1 $2 $3\n",
	"post-receive": "#!/usr/bin/env %s\n\"%s\" hook --config='%s' post-receive\n",
}

func createDelegateHooks(repoPath string) (err error) {
	for _, name := range git.ServerSideHooks {
		hookPath := filepath.Join(repoPath, "hooks", string(name))
		if err = os.WriteFile(hookPath,
			[]byte(fmt.Sprintf(hooksTpls[name], conf.Repository.ScriptType, conf.Repository.BashPath, "")),
			os.ModePerm); err != nil {
			return fmt.Errorf("create delegate hook '%s': %v", hookPath, err)
		}
		break
	}
	return nil
}
