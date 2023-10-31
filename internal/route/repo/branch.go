package repo

import (
	"fmt"
	"git-server/internal/conf"
	"git-server/internal/context"
	"git-server/internal/type"
	"github.com/gogs/git-module"
	"path/filepath"
	"time"
)

func Branches(c *context.Context) {

	branches, err := loadBranches(c)
	if err != nil {
		c.JSON(500, _type.FaildResult(err))
		return
	}

	now := time.Now()
	var DefaultBranch *Branch
	activeBranches := make([]*Branch, 0, 3)
	staleBranches := make([]*Branch, 0, 3)
	for i := range branches {
		switch {
		case branches[i].Name == c.Repo.BranchName:
			DefaultBranch = branches[i]
		case branches[i].Commit.Committer.When.Add(30 * 24 * time.Hour).After(now): // 30 days
			activeBranches = append(activeBranches, branches[i])
		case branches[i].Commit.Committer.When.Add(3 * 30 * 24 * time.Hour).Before(now): // 90 days
			staleBranches = append(staleBranches, branches[i])
		}
	}
	type result struct {
		DefaultBranch  *Branch
		ActiveBranches []*Branch
		StaleBranches  []*Branch
	}
	c.JSON(200, _type.SuccessResult(result{
		DefaultBranch:  DefaultBranch,
		ActiveBranches: activeBranches,
		StaleBranches:  staleBranches,
	}))
	c.Data["ActiveBranches"] = activeBranches
	c.Data["StaleBranches"] = staleBranches
}
func loadBranches(c *context.Context) ([]*Branch, error) {
	rawBranches := c.Data["Branches"].([]string)

	protectBranches, err := GetProtectedBranch(c)
	if err != nil {
		return nil, err
	}

	branches := make([]*Branch, len(rawBranches))
	repoPath := filepath.Join(conf.Repository.Root, c.Repo.RepoLink) + ".git"
	for i := range rawBranches {
		commit, err := GetCommit(rawBranches[i], repoPath)
		if err != nil {
			return nil, err
		}

		branches[i] = &Branch{
			Name:   rawBranches[i],
			Commit: commit,
		}

		for j := range protectBranches {
			if branches[i].Name == protectBranches[j] {
				branches[i].IsProtected = true
				break
			}
		}
	}

	return branches, nil
}
func GetCommit(branch, repoPath string) (*git.Commit, error) {
	gitRepo, err := git.Open(repoPath)
	if err != nil {
		return nil, fmt.Errorf("open repository: %v", err)
	}
	return gitRepo.BranchCommit(branch)
}
func AllBranches(c *context.Context) {
	branches, err := loadBranches(c)
	if err != nil {
		c.JSON(500, _type.FaildResult(err))
		return
	}
	var DefaultBranch *Branch
	OtherBranches := make([]*Branch, 0)
	for i := range branches {
		switch {
		case branches[i].Name == c.Repo.BranchName:
			DefaultBranch = branches[i]
		default:
			OtherBranches = append(OtherBranches, branches[i])
		}
	}
	type result struct {
		DefaultBranch *Branch
		OtherBranches []*Branch
	}
	c.JSON(200, _type.SuccessResult(result{
		DefaultBranch: DefaultBranch,
		OtherBranches: OtherBranches,
	}))
}
