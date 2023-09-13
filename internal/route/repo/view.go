package repo

import (
	"fmt"
	"git-server/internal/conf"
	"git-server/internal/context"
	"git-server/internal/type"
	"github.com/gogs/git-module"
	"net/http"
	"time"
)

func renderDirectory(c *context.Context, treeLink string) {
	tree, err := c.Repo.Commit.Subtree(c.Repo.TreePath)
	if err != nil {
		//get subtree
		c.JSON(500, _type.FaildResult(err))
		return
	}

	entries, err := tree.Entries()
	if err != nil {
		//list entries
		c.JSON(500, _type.FaildResult(err))
		return
	}
	entries.Sort()

	data, err := entries.CommitsInfo(c.Repo.Commit, git.CommitsInfoOptions{
		Path:           c.Repo.TreePath,
		MaxConcurrency: 0,
		Timeout:        5 * time.Minute,
	})
	if err != nil {
		//get commits info
		c.JSON(500, _type.FaildResult(err))
		return
	}
	latestCommit := c.Repo.Commit
	if len(c.Repo.TreePath) > 0 {
		latestCommit, err = c.Repo.Commit.CommitByPath(git.CommitByRevisionOptions{Path: c.Repo.TreePath})
		if err != nil {
			//get commit by path
			c.JSON(500, _type.FaildResult(err))
			return
		}
	}
	c.Data["LatestCommit"] = latestCommit
	res := struct {
		Branchs         []string
		LatestCommit    map[string]interface{}
		EntryCommitInfo []_type.EntryCommitInfo
	}{
		Branchs:         c.Data["Branches"].([]string),
		LatestCommit:    _type.ProduceLastCommit(latestCommit),
		EntryCommitInfo: _type.ProduceEntryCommitInfo(data),
	}
	c.JSON(200, _type.SuccessResult(res))

}

func renderFile(c *context.Context, entry *git.TreeEntry, treeLink, rawLink string) {
	blob := entry.Blob()
	data := struct {
		Url      string
		FileSize int64
		FileName string
	}{
		Url:      conf.Server.ExternalURL + rawLink + "/" + c.Repo.TreePath,
		FileSize: blob.Size(),
		FileName: blob.Name(),
	}
	c.JSON(http.StatusOK, _type.SuccessResult(data))

}

func Home(c *context.Context) {
	branchLink := c.Repo.RepoLink + "/src/" + c.Repo.BranchName
	treeLink := branchLink
	rawLink := c.Repo.RepoLink + "/raw/" + c.Repo.BranchName
	if len(c.Repo.TreePath) > 0 {
		treeLink += "/" + c.Repo.TreePath
	} else {
		var err error
		c.Repo.CommitsCount, err = c.Repo.Commit.CommitsCount()
		if err != nil {
			c.Error(500, "count commits")
			return
		}
		c.Data["CommitsCount"] = c.Repo.CommitsCount
	}
	// Get current entry user currently looking at.
	entry, err := c.Repo.Commit.TreeEntry(c.Repo.TreePath)
	if err != nil {
		fmt.Println("get tree entry")
		return
	}
	if entry.IsTree() {
		renderDirectory(c, treeLink)
	} else {
		renderFile(c, entry, treeLink, rawLink)
	}

	//var treeNames []string
	//paths := make([]string, 0, 5)
	//if len(c.Repo.TreePath) > 0 {
	//	treeNames = strings.Split(c.Repo.TreePath, "/")
	//	for i := range treeNames {
	//		paths = append(paths, strings.Join(treeNames[:i+1], "/"))
	//	}
	//
	//	c.Data["HasParentPath"] = true
	//	if len(paths)-2 >= 0 {
	//		c.Data["ParentPath"] = "/" + paths[len(paths)-2]
	//	}
	//}
}
