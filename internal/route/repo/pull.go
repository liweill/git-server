package repo

import (
	"fmt"
	"git-server/internal/conf"
	"git-server/internal/context"
	"git-server/internal/form"
	"git-server/internal/gitutil"
	processed "git-server/internal/process"
	"git-server/internal/type"
	"github.com/gogs/git-module"
	"github.com/pkg/errors"
	"github.com/unknwon/com"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MergeStyle represents the approach to merge commits into base branch.
type MergeStyle string

const (
	MERGE_STYLE_REGULAR MergeStyle = "create_merge_commit"
	MERGE_STYLE_REBASE  MergeStyle = "rebase_before_merging"
)

func MergePullRequest(c *context.Context, f form.PullRequest) {
	if err := Merge(f, c.Repo.GitRepo, MergeStyle(c.Query("merge_style")), c.Query("commit_description")); err != nil {
		c.JSON(500, _type.FaildResult(err))
		return
	}
	c.JSON(200, _type.SuccessResult("success"))
}
func Merge(f form.PullRequest, baseGitRepo *git.Repository, mergeStyle MergeStyle, commitDescription string) error {
	headRepoPath := filepath.Join(conf.Repository.Root, f.HeadRepo) + ".git"
	var err error
	// Create temporary directory to store temporary copy of the base repository,
	// and clean it up when operation finished regardless of succeed or not.
	tmpBasePath := filepath.Join(conf.Repository.LocalPath, "data", com.ToStr(time.Now().Nanosecond())+".git")
	if err := os.MkdirAll(filepath.Dir(tmpBasePath), os.ModePerm); err != nil {
		return err
	}
	defer func() {
		_ = os.RemoveAll(filepath.Dir(tmpBasePath))
	}()

	// Clone the base repository to the defined temporary directory,
	// and checks out to base branch directly.
	var stderr string
	if _, stderr, err = processed.ExecTimeout(5*time.Minute,
		fmt.Sprintf("PullRequest.Merge (git clone): %s", tmpBasePath),
		"git", "clone", "-b", f.BaseBranch, baseGitRepo.Path(), tmpBasePath); err != nil {
		return fmt.Errorf("git clone: %s", stderr)
	}
	fmt.Println("headRepoPath", headRepoPath)

	// Add remote which points to the head repository.
	if _, stderr, err = processed.ExecDir(-1, tmpBasePath,
		fmt.Sprintf("PullRequest.Merge (git remote add): %s", tmpBasePath),
		"git", "remote", "add", "head_repo", headRepoPath); err != nil {
		return fmt.Errorf("git remote add [%s -> %s]: %s", headRepoPath, tmpBasePath, stderr)
	}

	// Fetch information from head repository to the temporary copy.
	if _, stderr, err = processed.ExecDir(-1, tmpBasePath,
		fmt.Sprintf("PullRequest.Merge (git fetch): %s", tmpBasePath),
		"git", "fetch", "head_repo"); err != nil {
		return fmt.Errorf("git fetch [%s -> %s]: %s", headRepoPath, tmpBasePath, stderr)
	}

	remoteHeadBranch := "head_repo/" + f.HeadBranch

	// Check if merge style is allowed, reset to default style if not
	//if mergeStyle == MERGE_STYLE_REBASE && !f.BaseRepo.PullsAllowRebase {
	//	mergeStyle = MERGE_STYLE_REGULAR
	//}

	switch mergeStyle {
	case MERGE_STYLE_REGULAR: // Create merge commit

		// Merge changes from head branch.
		if _, stderr, err = processed.ExecDir(-1, tmpBasePath,
			fmt.Sprintf("PullRequest.Merge (git merge --no-ff --no-commit): %s", tmpBasePath),
			"git", "merge", "--no-ff", "--no-commit", remoteHeadBranch); err != nil {
			return fmt.Errorf("git merge --no-ff --no-commit [%s]: %v - %s", tmpBasePath, err, stderr)
		}

		//new 13684856438 repo
		//fmt.Println("HeadBranch,HeadUserName,HeadRepo.Name", pr.HeadBranch, pr.HeadUserName, pr.HeadRepo.Name)
		// Create a merge commit for the base branch.
		if _, stderr, err = processed.ExecDir(-1, tmpBasePath,
			fmt.Sprintf("PullRequest.Merge (git merge): %s", tmpBasePath),
			"git", "commit", fmt.Sprintf("--author='%s <%s>'", f.UserName, "1571334850@qq.com"),
			"-m", fmt.Sprintf("Merge branch '%s' of %s into %s", f.HeadBranch, f.HeadRepo, f.BaseBranch),
			"-m", commitDescription); err != nil {
			return fmt.Errorf("git commit [%s]: %v - %s", tmpBasePath, err, stderr)
		}

	case MERGE_STYLE_REBASE: // Rebase before merging

		// Rebase head branch based on base branch, this creates a non-branch commit state.
		if _, stderr, err = processed.ExecDir(-1, tmpBasePath,
			fmt.Sprintf("PullRequest.Merge (git rebase): %s", tmpBasePath),
			"git", "rebase", "--quiet", f.BaseBranch, remoteHeadBranch); err != nil {
			return fmt.Errorf("git rebase [%s on %s]: %s", remoteHeadBranch, f.BaseBranch, stderr)
		}

		// Name non-branch commit state to a new temporary branch in order to save changes.
		tmpBranch := com.ToStr(time.Now().UnixNano(), 10)
		if _, stderr, err := processed.ExecDir(-1, tmpBasePath,
			fmt.Sprintf("PullRequest.Merge (git checkout): %s", tmpBasePath),
			"git", "checkout", "-b", tmpBranch); err != nil {
			return fmt.Errorf("git checkout '%s': %s", tmpBranch, stderr)
		}

		// Check out the base branch to be operated on.
		if _, stderr, err = processed.ExecDir(-1, tmpBasePath,
			fmt.Sprintf("PullRequest.Merge (git checkout): %s", tmpBasePath),
			"git", "checkout", f.BaseBranch); err != nil {
			return fmt.Errorf("git checkout '%s': %s", f.BaseBranch, stderr)
		}

		// Merge changes from temporary branch to the base branch.
		if _, stderr, err = processed.ExecDir(-1, tmpBasePath,
			fmt.Sprintf("PullRequest.Merge (git merge): %s", tmpBasePath),
			"git", "merge", tmpBranch); err != nil {
			return fmt.Errorf("git merge [%s]: %v - %s", tmpBasePath, err, stderr)
		}

	default:
		return fmt.Errorf("unknown merge style: %s", mergeStyle)
	}

	// Push changes on base branch to upstream.
	if _, stderr, err = processed.ExecDir(-1, tmpBasePath,
		fmt.Sprintf("PullRequest.Merge (git push): %s", tmpBasePath),
		"git", "push", baseGitRepo.Path(), f.BaseBranch); err != nil {
		return fmt.Errorf("git push: %s", stderr)
	}
	return nil
}
func CompareAndPullRequest(c *context.Context) {
	headGitRepo, prInfo, headBranch, err := ParseCompareInfo(c)
	if err != nil {
		c.JSON(500, _type.FaildResult(err))
		return
	}
	headBranches := c.Data["HeadBranches"].([]string)
	baseBranches := c.Data["Branches"].([]string)
	nothingToCompare, diff, commits, err := PrepareCompareDiff(c, headGitRepo, prInfo, headBranch)
	if err != nil {
		c.JSON(500, _type.FaildResult(err))
		return
	}
	type Commit map[string]interface{}
	type data struct {
		IsCompare      bool
		Commit         []Commit
		Diff           *DiffInfo
		HeadBranches   []string
		BaseBranches   []string
		BeforeCommitID string
		AfterCommitID  string
	}
	if nothingToCompare {
		// Setup information for new form.
		c.JSON(200, _type.SuccessResult(data{
			IsCompare:    false,
			HeadBranches: headBranches,
			BaseBranches: baseBranches,
		}))
		return
	}
	change := Change{
		TotalAdditions: diff.TotalAdditions(),
		TotalDeletions: diff.TotalDeletions(),
		IsIncomplete:   diff.IsIncomplete(),
	}
	diffInfo := &DiffInfo{
		Changes: change,
		Files:   diff.Files,
	}
	Commits := make([]Commit, 0)
	for i := 0; i < len(commits); i++ {
		Commits = append(Commits, _type.ProduceLastCommit(commits[i]))
	}
	c.JSON(200, _type.SuccessResult(data{
		IsCompare:      true,
		HeadBranches:   headBranches,
		BaseBranches:   baseBranches,
		Commit:         Commits,
		Diff:           diffInfo,
		BeforeCommitID: c.Data["BeforeCommitID"].(string),
		AfterCommitID:  c.Data["AfterCommitID"].(string),
	}))

}
func ParseCompareInfo(c *context.Context) (*git.Repository, *gitutil.PullRequestMeta, string, error) {

	// Get compared branches information
	// format: <base branch>...[<head repo>:]<head branch>
	// base<-head: master...head:feature
	// same repo: master...feature
	//infos := strings.Split(c.Params("*"), "...")
	//
	//if len(infos) != 2 {
	//	log.Trace("ParseCompareInfo[%d]: not enough compared branches information %s", "", infos)
	//	c.NotFound()
	//	return nil, nil, ""
	//}

	baseBranch := c.Params("before")
	c.Data["BaseBranch"] = baseBranch

	var (
		headBranch string
		headPepo   string
		headUser   string
		isSameRepo bool
		err        error
	)

	// If there is no head repository, it means pull request between same repository.
	headInfos := strings.Split(c.Params("after"), ":")
	if len(headInfos) == 1 {
		isSameRepo = true
		headBranch = headInfos[0]

	} else if len(headInfos) == 3 {
		headUser = headInfos[0]
		headPepo = headInfos[1]
		headBranch = headInfos[2]
		isSameRepo = false

	} else {
		return nil, nil, "", errors.New("status.page_not_found")
	}
	c.Repo.PullRequest.SameRepo = isSameRepo

	// Check if base branch is valid.
	if !c.Repo.GitRepo.HasBranch(baseBranch) {
		return nil, nil, "", errors.New("no branch")
	}

	var headGitRepo *git.Repository

	// In case user included redundant head user name for comparison in same repository,
	// no need to check the fork relation.
	if !isSameRepo {
		headRepoPath := filepath.Join(conf.Repository.Root, headUser, headPepo) + ".git"
		headGitRepo, err = git.Open(headRepoPath)
		if err != nil {
			return nil, nil, "", errors.New("open repository err")
		}
	} else {
		headGitRepo = c.Repo.GitRepo
	}

	//if !db.Perms.Authorize(
	//	c.Req.Context(),
	//	c.User.ID,
	//	headRepo.ID,
	//	db.AccessModeWrite,
	//	db.AccessModeOptions{
	//		OwnerID: headRepo.OwnerID,
	//		Private: headRepo.IsPrivate,
	//	},
	//) && !c.User.IsAdmin {
	//	log.Trace("ParseCompareInfo [base_repo_id: %d]: does not have write access or site admin", baseRepo.ID)
	//	c.NotFound()
	//	return nil, nil, "", ""
	//}

	// Check if head branch is valid.
	if !headGitRepo.HasBranch(headBranch) {
		c.NotFound()
		return nil, nil, "", errors.New("no branch")
	}

	headBranches, err := headGitRepo.Branches()
	if err != nil {
		c.Error(500, "get branches")
		return nil, nil, "", errors.New("get branches err")
	}
	c.Data["HeadBranches"] = headBranches

	baseRepoPath := filepath.Join(conf.Repository.Root, c.Params(":username"), c.Params(":reponame")) + ".git"
	meta, err := gitutil.Module.PullRequestMeta(headGitRepo.Path(), baseRepoPath, headBranch, baseBranch)
	if err != nil {
		if gitutil.IsErrNoMergeBase(err) {
			c.Data["IsNoMergeBase"] = true
		} else {
			c.Error(500, "get pull request meta")
		}
		return nil, nil, "", err
	}
	c.Data["BeforeCommitID"] = meta.MergeBase

	return headGitRepo, meta, headBranch, nil
}
func PrepareCompareDiff(
	c *context.Context,
	headGitRepo *git.Repository,
	meta *gitutil.PullRequestMeta,
	headBranch string,
) (bool, *gitutil.Diff, []*git.Commit, error) {
	var (
		err error
	)

	// Get diff information.

	headCommitID, err := headGitRepo.BranchCommitID(headBranch)
	if err != nil {
		return false, nil, nil, errors.New("get head branch commit ID err")
	}
	c.Data["AfterCommitID"] = headCommitID

	if headCommitID == meta.MergeBase {
		c.Data["IsNothingToCompare"] = true
		return true, nil, nil, nil
	}

	diff, err := gitutil.RepoDiff(headGitRepo,
		headCommitID, conf.Git.MaxDiffFiles, conf.Git.MaxDiffLines, conf.Git.MaxDiffLineChars,
		git.DiffOptions{Base: meta.MergeBase, Timeout: time.Duration(conf.Git.Timeout.Diff) * time.Second},
	)
	if err != nil {
		return false, nil, nil, errors.New("get repository diff err")
	}
	c.Data["Diff"] = diff
	c.Data["DiffNotAvailable"] = diff.NumFiles() == 0

	headCommit, err := headGitRepo.CatFileCommit(headCommitID)
	if err != nil {
		return false, nil, nil, errors.New("get head commit err")
	}

	//	c.Data["Commits"] = matchUsersWithCommitEmails(c.Req.Context(), meta.Commits)
	c.Data["CommitCount"] = len(meta.Commits)
	c.Data["IsImageFile"] = headCommit.IsImageFile
	c.Data["IsImageFileByIndex"] = headCommit.IsImageFileByIndex

	return false, diff, meta.Commits, nil
}
func ViewPullCommits(c *context.Context) {

}
