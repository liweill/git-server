package repo

import (
	"errors"
	"fmt"
	"git-server/internal/conf"
	"git-server/internal/context"
	"git-server/internal/gitutil"
	"git-server/internal/tool"
	"git-server/internal/type"
	"github.com/gogs/git-module"
	"time"
)

type DiffInfo struct {
	Changes Change
	Files   []*gitutil.DiffFile
}

type Change struct {
	TotalAdditions int
	TotalDeletions int
	IsIncomplete   bool
}

func Diff(c *context.Context) {

	commitID := c.Params(":sha")

	commit, err := c.Repo.GitRepo.CatFileCommit(commitID)
	if err != nil {
		c.JSON(500, _type.FaildResult(errors.New(fmt.Sprintf("%v %s", err, "get commit by ID"))))
		return
	}

	diff, err := gitutil.RepoDiff(c.Repo.GitRepo,
		commitID, conf.Git.MaxDiffFiles, conf.Git.MaxDiffLines, conf.Git.MaxDiffLineChars,
		git.DiffOptions{Timeout: time.Duration(conf.Git.Timeout.Diff) * time.Second},
	)

	change := Change{
		TotalAdditions: diff.TotalAdditions(),
		TotalDeletions: diff.TotalDeletions(),
		IsIncomplete:   diff.IsIncomplete(),
	}

	diffInfo := &DiffInfo{
		Changes: change,
		Files:   diff.Files,
	}

	if err != nil {
		c.JSON(500, _type.FaildResult(errors.New(fmt.Sprintf("%v %s", err, "get diff"))))
		return
	}

	parents := make([]string, commit.ParentsCount())
	for i := 0; i < commit.ParentsCount(); i++ {
		sha, err := commit.ParentID(i)
		if err != nil {
			c.JSON(500, _type.FaildResult(errors.New("status.page_not_found")))
			return
		}
		parents[i] = sha.String()
	}

	//setEditorconfigIfExists(c)
	if c.Written() {
		return
	}

	c.RawTitle(commit.Summary() + " · " + tool.ShortSHA1(commitID))
	c.Data["CommitID"] = commitID
	c.Data["IsSplitStyle"] = c.Query("style") == "split"
	c.Data["IsImageFile"] = commit.IsImageFile
	c.Data["IsImageFileByIndex"] = commit.IsImageFileByIndex
	c.Data["Commit"] = commit
	//c.Data["Author"] = tryGetUserByEmail(c.Req.Context(), commit.Author.Email)
	c.Data["Diff"] = diff
	c.Data["Parents"] = parents
	c.Data["DiffNotAvailable"] = diff.NumFiles() == 0
	data := struct {
		Commit  map[string]interface{}
		Diff    *DiffInfo
		Parents []string
	}{
		Commit:  _type.ProduceLastCommit(commit),
		Diff:    diffInfo,
		Parents: parents,
	}
	c.JSON(200, _type.SuccessResult(data))
	//c.Data["SourcePath"] = conf.Server.Subpath + "/" + path.Join(userName, repoName, "src", commitID)
	//c.Data["RawPath"] = conf.Server.Subpath + "/" + path.Join(userName, repoName, "raw", commitID)
	//if commit.ParentsCount() > 0 {
	//	c.Data["BeforeSourcePath"] = conf.Server.Subpath + "/" + path.Join(userName, repoName, "src", parents[0])
	//	c.Data["BeforeRawPath"] = conf.Server.Subpath + "/" + path.Join(userName, repoName, "raw", parents[0])
	//}
}
func RefCommits(c *context.Context) {
	c.Data["PageIsViewFiles"] = true
	switch {
	case c.Repo.TreePath == "":
		Commits(c)
	case c.Repo.TreePath == "search":
		SearchCommits(c)
	default:
		FileHistory(c)
	}
}
func Commits(c *context.Context) {
	renderCommits(c, "")
}
func renderCommits(c *context.Context, filename string) {
	page := c.QueryInt("page")
	if page < 1 {
		page = 1
	}
	pageSize := c.QueryInt("pageSize")
	if pageSize < 1 {
		pageSize = 5
	}

	commits, err := c.Repo.Commit.CommitsByPage(page, pageSize, git.CommitsByPageOptions{Path: filename})
	if err != nil {
		c.JSON(500, _type.FaildResult(errors.New("paging commits")))
		return
	}

	commits = RenderIssueLinks(commits, c.Repo.RepoLink)
	type Commit map[string]interface{}
	Commits := make([]Commit, 0)
	for i := 0; i < len(commits); i++ {
		Commits = append(Commits, _type.ProduceLastCommit(commits[i]))
	}
	c.JSON(200, _type.SuccessResult(Commits))

}

// TODO(unknwon)
func RenderIssueLinks(oldCommits []*git.Commit, _ string) []*git.Commit {
	return oldCommits
}

func SearchCommits(c *context.Context) {
	c.Data["PageIsCommits"] = true

	keyword := c.Query("q")
	if keyword == "" {
		c.Redirect(c.Repo.RepoLink + "/commits/" + c.Repo.BranchName)
		return
	}

	commits, err := c.Repo.Commit.SearchCommits(keyword)
	if err != nil {
		c.JSON(500, _type.FaildResult(errors.New("search commits")))
		return
	}
	commits = RenderIssueLinks(commits, c.Repo.RepoLink)
	type Commit map[string]interface{}
	Commits := make([]Commit, 0)
	for i := 0; i < len(commits); i++ {
		Commits = append(Commits, _type.ProduceLastCommit(commits[i]))
	}
	c.JSON(200, _type.SuccessResult(Commits))

}
func FileHistory(c *context.Context) {
	renderCommits(c, c.Repo.TreePath)
}
