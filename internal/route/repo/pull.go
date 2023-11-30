package repo

import (
	"fmt"
	"git-server/internal/conf"
	"git-server/internal/context"
	"git-server/internal/form"
	"git-server/internal/gitutil"
	"git-server/internal/osutil"
	processed "git-server/internal/process"
	"git-server/internal/type"
	"github.com/gogs/git-module"
	"github.com/pkg/errors"
	"github.com/unknwon/com"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
	log "unknwon.dev/clog/v2"
)

// MergeStyle represents the approach to merge commits into base branch.
type MergeStyle string

const (
	MERGE_STYLE_REGULAR MergeStyle = "create_merge_commit"
	MERGE_STYLE_REBASE  MergeStyle = "rebase_before_merging"
)

type PullRequestStatus int

const (
	PULL_REQUEST_STATUS_CONFLICT PullRequestStatus = iota
	PULL_REQUEST_STATUS_CHECKING
	PULL_REQUEST_STATUS_MERGEABLE
)

type NumInfo struct {
	NumCommits int
	NumFiles   int
}

func MergePullRequest(c *context.Context, f form.MergePullRequest) {
	var (
		MergedCommitID string
		err            error
	)
	if MergedCommitID, err = Merge(f.Pull, c.Repo.GitRepo, MergeStyle(c.Query("merge_style")), c.Query("commit_description")); err != nil {
		c.JSON(500, _type.FaildResult(err))
		return
	}
	err = checkAndUpdateStatus(&f)
	if err != nil {
		c.JSON(500, errors.Errorf("checkAndUpdateStatus err:%v", err))
	}
	f.Pull.MergeCommitId = MergedCommitID
	f.Pull.HasMerged = true
	f.Pull.IsClosed = true
	c.JSON(200, _type.SuccessResult(f))
}
func checkAndUpdateStatus(f *form.MergePullRequest) error {
	for i := 0; i < len(f.Pulls); i++ {
		status, err := testPatch(f.Pulls[i].IssueId, f.Pulls[i].BaseRepo, f.Pulls[i].BaseBranch)
		if err != nil {
			return err
		}
		// No conflict appears after test means mergeable.
		if status == PULL_REQUEST_STATUS_CHECKING {
			status = PULL_REQUEST_STATUS_MERGEABLE
		}
		f.Pulls[i].Status = int(status)
	}
	return nil
}
func Merge(f form.PullRequest, baseGitRepo *git.Repository, mergeStyle MergeStyle, commitDescription string) (string, error) {
	headRepoPath := filepath.Join(conf.Repository.Root, f.HeadRepo) + ".git"
	var err error
	// Create temporary directory to store temporary copy of the base repository,
	// and clean it up when operation finished regardless of succeed or not.
	tmpBasePath := filepath.Join(conf.Repository.LocalPath, "data", com.ToStr(time.Now().Nanosecond())+".git")
	if err := os.MkdirAll(filepath.Dir(tmpBasePath), os.ModePerm); err != nil {
		return "", err
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
		return "", fmt.Errorf("git clone: %s", stderr)
	}
	headGitRepo, err := git.Open(headRepoPath)
	if err != nil {
		return "", err
	}
	// Add remote which points to the head repository.
	if _, stderr, err = processed.ExecDir(-1, tmpBasePath,
		fmt.Sprintf("PullRequest.Merge (git remote add): %s", tmpBasePath),
		"git", "remote", "add", "head_repo", headRepoPath); err != nil {
		return "", fmt.Errorf("git remote add [%s -> %s]: %s", headRepoPath, tmpBasePath, stderr)
	}

	// Fetch information from head repository to the temporary copy.
	if _, stderr, err = processed.ExecDir(-1, tmpBasePath,
		fmt.Sprintf("PullRequest.Merge (git fetch): %s", tmpBasePath),
		"git", "fetch", "head_repo"); err != nil {
		return "", fmt.Errorf("git fetch [%s -> %s]: %s", headRepoPath, tmpBasePath, stderr)
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
			return "", fmt.Errorf("git merge --no-ff --no-commit [%s]: %v - %s", tmpBasePath, err, stderr)
		}

		//new 13684856438 repo
		//fmt.Println("HeadBranch,HeadUserName,HeadRepo.Name", pr.HeadBranch, pr.HeadUserName, pr.HeadRepo.Name)
		// Create a merge commit for the base branch.
		if _, stderr, err = processed.ExecDir(-1, tmpBasePath,
			fmt.Sprintf("PullRequest.Merge (git merge): %s", tmpBasePath),
			"git", "commit", fmt.Sprintf("--author='%s <%s>'", f.UserName, "1571334850@qq.com"),
			"-m", fmt.Sprintf("Merge branch '%s' of %s into %s", f.HeadBranch, f.HeadRepo, f.BaseBranch),
			"-m", commitDescription); err != nil {
			return "", fmt.Errorf("git commit [%s]: %v - %s", tmpBasePath, err, stderr)
		}

	case MERGE_STYLE_REBASE: // Rebase before merging

		// Rebase head branch based on base branch, this creates a non-branch commit state.
		if _, stderr, err = processed.ExecDir(-1, tmpBasePath,
			fmt.Sprintf("PullRequest.Merge (git rebase): %s", tmpBasePath),
			"git", "rebase", "--quiet", f.BaseBranch, remoteHeadBranch); err != nil {
			return "", fmt.Errorf("git rebase [%s on %s]: %s", remoteHeadBranch, f.BaseBranch, stderr)
		}

		// Name non-branch commit state to a new temporary branch in order to save changes.
		tmpBranch := com.ToStr(time.Now().UnixNano(), 10)
		if _, stderr, err := processed.ExecDir(-1, tmpBasePath,
			fmt.Sprintf("PullRequest.Merge (git checkout): %s", tmpBasePath),
			"git", "checkout", "-b", tmpBranch); err != nil {
			return "", fmt.Errorf("git checkout '%s': %s", tmpBranch, stderr)
		}

		// Check out the base branch to be operated on.
		if _, stderr, err = processed.ExecDir(-1, tmpBasePath,
			fmt.Sprintf("PullRequest.Merge (git checkout): %s", tmpBasePath),
			"git", "checkout", f.BaseBranch); err != nil {
			return "", fmt.Errorf("git checkout '%s': %s", f.BaseBranch, stderr)
		}

		// Merge changes from temporary branch to the base branch.
		if _, stderr, err = processed.ExecDir(-1, tmpBasePath,
			fmt.Sprintf("PullRequest.Merge (git merge): %s", tmpBasePath),
			"git", "merge", tmpBranch); err != nil {
			return "", fmt.Errorf("git merge [%s]: %v - %s", tmpBasePath, err, stderr)
		}

	default:
		return "", fmt.Errorf("unknown merge style: %s", mergeStyle)
	}

	// Push changes on base branch to upstream.
	if _, stderr, err = processed.ExecDir(-1, tmpBasePath,
		fmt.Sprintf("PullRequest.Merge (git push): %s", tmpBasePath),
		"git", "push", baseGitRepo.Path(), f.BaseBranch); err != nil {
		return "", fmt.Errorf("git push: %s", stderr)
	}
	MergedCommitID, err := headGitRepo.BranchCommitID(f.HeadBranch)

	return MergedCommitID, nil
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
		SourcePath     string
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
		SourcePath:     c.Data["SourcePath"].(string),
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
	c.Data["SourcePath"] = conf.Server.ExternalURL + path.Join(c.Repo.RepoLink, "raw", headCommitID)
	return false, diff, meta.Commits, nil
}
func ViewPullCommits(c *context.Context, f form.PullRequest) {
	var (
		commits []*git.Commit
		numInfo NumInfo
		err     error
	)

	if f.HasMerged {
		numInfo, err = PrepareMergedViewPullInfo(c, f)
		if err != nil {
			c.JSON(500, _type.FaildResult(err))
			return
		}
		startCommit, err := c.Repo.GitRepo.CatFileCommit(f.MergeBase)
		if err != nil {
			c.JSON(500, _type.FaildResult(errors.Errorf("get commit of merge base:%v", err)))
			return
		}
		endCommit, err := c.Repo.GitRepo.CatFileCommit(f.MergeCommitId)
		if err != nil {
			c.Error(500, "get merged commit")
			return
		}
		commits, err = c.Repo.GitRepo.RevList([]string{startCommit.ID.String() + "..." + endCommit.ID.String()})
		if err != nil {
			c.Error(500, "list commits")
			return
		}

	} else {
		prInfo, err := ViewPullInfo(f)
		if err != nil {
			c.JSON(500, _type.FaildResult(err))
			return
		}
		if prInfo == nil {
			c.JSON(500, _type.FaildResult(errors.New("Not Found")))
			return
		}
		commits = prInfo.Commits
		numInfo = NumInfo{
			NumCommits: len(prInfo.Commits),
			NumFiles:   prInfo.NumFiles,
		}
	}
	type Commit map[string]interface{}
	Commits := make([]Commit, 0)
	for i := 0; i < len(commits); i++ {
		Commits = append(Commits, _type.ProduceLastCommit(commits[i]))
	}
	type Info struct {
		Commits []Commit
		NumInfo NumInfo
	}
	c.JSON(200, _type.SuccessResult(Info{
		Commits: Commits,
		NumInfo: numInfo,
	}))
}
func PrepareMergedViewPullInfo(c *context.Context, f form.PullRequest) (NumInfo, error) {

	var err error
	NumCommits, err := c.Repo.GitRepo.RevListCount([]string{f.MergeBase + "..." + f.MergeCommitId})
	if err != nil {
		c.Error(500, "count commits")
		return NumInfo{}, err
	}

	names, err := c.Repo.GitRepo.DiffNameOnly(f.MergeBase, f.MergeCommitId, git.DiffNameOnlyOptions{NeedsMergeBase: true})
	NumFiles := len(names)
	if err != nil {
		c.Error(500, "get changed files")
		return NumInfo{}, err
	}
	return NumInfo{
		NumCommits: int(NumCommits),
		NumFiles:   NumFiles,
	}, nil
}
func CompareAndPullRequestPost(c *context.Context) {
	headGitRepo, meta, headBranch, err := ParseCompareInfo(c)
	MergeBase := meta.MergeBase
	type info struct {
		MergeBase string
		Status    int
		IssueId   int
	}
	baseBranch := c.Params("before")
	if err != nil {
		c.JSON(500, _type.FaildResult(err))
		return
	}
	patch, err := headGitRepo.DiffBinary(meta.MergeBase, headBranch)
	if err != nil {
		c.Error(500, "get patch")
		return
	}
	index, err := getIndex(c)
	if err != nil {
		c.JSON(500, _type.FaildResult(err))
		return
	}
	if err = SavePatch(index, patch, c.Repo.RepoLink); err != nil {
		c.JSON(500, _type.FaildResult(errors.Errorf("SavePatch err:%v", err)))
		return
	}
	var status PullRequestStatus
	if status, err = testPatch(index, c.Repo.RepoLink, baseBranch); err != nil {
		c.JSON(500, _type.FaildResult(errors.Errorf("testPatch: %v", err)))
		return
	}
	// No conflict appears after test means mergeable.
	if status == PULL_REQUEST_STATUS_CHECKING {
		status = PULL_REQUEST_STATUS_MERGEABLE
	}
	c.JSON(200, _type.SuccessResult(info{
		MergeBase: MergeBase,
		Status:    int(status),
		IssueId:   index,
	}))
}
func getIndex(c *context.Context) (int, error) {
	dir := filepath.Join(conf.Repository.Root, c.Repo.RepoLink) + ".git"
	dir = filepath.Join(dir, "pulls")
	_, err := os.Stat(dir)
	if err == nil {
		files, err := ioutil.ReadDir(dir)
		if err != nil {
			return 0, errors.New("读取目录出错")
		}
		count := 0
		for _, file := range files {
			if !file.IsDir() {
				count++
			}
		}
		return count + 1, nil
	} else if os.IsNotExist(err) {
		return 1, nil
	}
	return 0, errors.New("其他错误")
}

// SavePatch saves patch data to corresponding location by given issue ID.
func SavePatch(index int, patch []byte, repoLink string) error {
	patchPath, err := PatchPath(index, repoLink)
	if err != nil {
		return fmt.Errorf("PatchPath: %v", err)
	}

	if err = os.MkdirAll(filepath.Dir(patchPath), os.ModePerm); err != nil {
		return err
	}
	if err = os.WriteFile(patchPath, patch, 0644); err != nil {
		return fmt.Errorf("WriteFile: %v", err)
	}

	return nil
}

// PatchPath returns corresponding patch file path of repository by given issue ID.
func PatchPath(index int, repoLink string) (string, error) {
	path := filepath.Join(conf.Repository.Root, repoLink) + ".git"
	return filepath.Join(path, "pulls", com.ToStr(index)+".patch"), nil
}

// testPatch checks if patch can be merged to base repository without conflict.
// FIXME: make a mechanism to clean up stable local copies.
func testPatch(index int, repoLink string, baseBranch string) (PullRequestStatus, error) {

	patchPath, err := PatchPath(index, repoLink)
	if err != nil {
		return -1, fmt.Errorf("BaseRepo.PatchPath: %v", err)
	}

	// Fast fail if patch does not exist, this assumes data is corrupted.
	if !osutil.IsFile(patchPath) {
		log.Trace("PullRequest[%d].testPatch: ignored corrupted data")
		return -1, nil
	}

	repoWorkingPool.CheckIn(com.ToStr(repoLink))
	defer repoWorkingPool.CheckOut(com.ToStr(repoLink))

	log.Trace("PullRequest[%d].testPatch (patchPath): %s", "", patchPath)

	if err := UpdateLocalCopyBranch(repoLink, baseBranch); err != nil {
		return -1, fmt.Errorf("UpdateLocalCopy [%d]: %v", 1, err)
	}

	args := []string{"apply", "--check"}
	//if pr.BaseRepo.PullsIgnoreWhitespace {
	//	args = append(args, "--ignore-whitespace")
	//}
	args = append(args, patchPath)

	Status := PULL_REQUEST_STATUS_CHECKING
	_, stderr, err := processed.ExecDir(-1, LocalCopyPath(repoLink),
		fmt.Sprintf("testPatch (git apply --check): %s", repoLink),
		"git", args...)
	if err != nil {
		log.Trace("PullRequest[%d].testPatch (apply): has conflict\n%s", 1, stderr)
		Status = PULL_REQUEST_STATUS_CONFLICT
		return Status, nil
	}
	return Status, nil
}
func PrepareViewPullInfo(c *context.Context, f form.PullRequest) {
	var (
		numInfo NumInfo
		err     error
	)
	if f.HasMerged {
		numInfo, err = PrepareMergedViewPullInfo(c, f)
		if err != nil {
			c.JSON(500, _type.FaildResult(err))
			return
		}
	} else {
		prMeta, err := ViewPullInfo(f)
		if err != nil {
			c.JSON(500, _type.FaildResult(err))
			return
		}
		if prMeta == nil {
			numInfo = NumInfo{
				NumCommits: 0,
				NumFiles:   0,
			}
		} else {
			numInfo = NumInfo{
				NumCommits: len(prMeta.Commits),
				NumFiles:   prMeta.NumFiles,
			}
		}
	}
	c.JSON(200, numInfo)
}
func ViewPullInfo(f form.PullRequest) (*gitutil.PullRequestMeta, error) {
	baseRepoPath := filepath.Join(conf.Repository.Root, f.BaseRepo) + ".git"
	headRepoPath := filepath.Join(conf.Repository.Root, f.HeadRepo) + ".git"
	var (
		headGitRepo *git.Repository
		err         error
	)

	if f.HeadRepo != "" {
		headGitRepo, err = git.Open(headRepoPath)
		if err != nil {
			return nil, errors.Errorf("open repository:%v", err)
		}
	}

	if f.HeadRepo == "" || !headGitRepo.HasBranch(f.HeadBranch) {
		return nil, errors.New("no branch")
	}
	prMeta, err := gitutil.Module.PullRequestMeta(headRepoPath, baseRepoPath, f.HeadBranch, f.BaseBranch)
	if err != nil {
		if strings.Contains(err.Error(), "fatal: Not a valid object name") {
			return nil, nil
		}
		return nil, errors.New("err")
	}
	return prMeta, nil
}
func ViewPullFiles(c *context.Context, f form.PullRequest) {
	var (
		diffGitRepo   *git.Repository
		startCommitID string
		endCommitID   string
		gitRepo       *git.Repository
		numInfo       NumInfo
		err           error
	)

	if f.HasMerged {
		numInfo, err = PrepareMergedViewPullInfo(c, f)
		if err != nil {
			c.JSON(500, _type.FaildResult(err))
			return
		}

		diffGitRepo = c.Repo.GitRepo
		startCommitID = f.MergeBase
		endCommitID = f.MergeCommitId
		gitRepo = c.Repo.GitRepo
	} else {
		prInfo, err := ViewPullInfo(f)
		if err != nil {
			c.JSON(500, _type.FaildResult(err))
			return
		}
		if prInfo == nil {
			c.JSON(500, _type.FaildResult(errors.New("Not Found")))
			return
		}
		numInfo = NumInfo{
			NumCommits: len(prInfo.Commits),
			NumFiles:   prInfo.NumFiles,
		}

		headRepoPath := filepath.Join(conf.Repository.Root, f.HeadRepo) + ".git"

		headGitRepo, err := git.Open(headRepoPath)
		if err != nil {
			c.Error(500, "open repository")
			return
		}

		headCommitID, err := headGitRepo.BranchCommitID(f.HeadBranch)
		if err != nil {
			c.Error(500, "get head branch commit ID")
			return
		}

		diffGitRepo = headGitRepo
		startCommitID = prInfo.MergeBase
		endCommitID = headCommitID
		gitRepo = headGitRepo
	}

	diff, err := gitutil.RepoDiff(diffGitRepo,
		endCommitID, conf.Git.MaxDiffFiles, conf.Git.MaxDiffLines, conf.Git.MaxDiffLineChars,
		git.DiffOptions{Base: startCommitID, Timeout: time.Duration(conf.Git.Timeout.Diff) * time.Second},
	)
	if err != nil {
		c.Error(500, "get diff")
		return
	}
	c.Data["Diff"] = diff
	c.Data["DiffNotAvailable"] = diff.NumFiles() == 0

	commit, err := gitRepo.CatFileCommit(endCommitID)
	if err != nil {
		c.Error(500, "get commit")
		return
	}
	fmt.Println(commit, numInfo)
	type data struct {
		NumInfo    NumInfo
		Diff       *DiffInfo
		SourcePath string
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
	c.JSON(200, _type.SuccessResult(data{
		NumInfo:    numInfo,
		Diff:       diffInfo,
		SourcePath: conf.Server.ExternalURL + path.Join(c.Repo.RepoLink, "raw", endCommitID),
	}))

	// It is possible head repo has been deleted for merged pull requests
	//if pull.HeadRepo != nil {
	//	c.Data["Username"] = pull.HeadUserName
	//	c.Data["Reponame"] = pull.HeadRepo.Name
	//
	//	headTarget := path.Join(pull.HeadUserName, pull.HeadRepo.Name)
	//	c.Data["SourcePath"] = conf.Server.Subpath + "/" + path.Join(headTarget, "src", endCommitID)
	//	c.Data["RawPath"] = conf.Server.Subpath + "/" + path.Join(headTarget, "raw", endCommitID)
	//	c.Data["BeforeSourcePath"] = conf.Server.Subpath + "/" + path.Join(headTarget, "src", startCommitID)
	//	c.Data["BeforeRawPath"] = conf.Server.Subpath + "/" + path.Join(headTarget, "raw", startCommitID)
	//}
	//
	//c.Data["RequireHighlightJS"] = true
	//c.Success(PULL_FILES)
}
func MM(c *context.Context, f form.MergePullRequest) {
	fmt.Printf("ss:%+v\n", f.Pull)
	for i := 0; i < len(f.Pulls); i++ {
		fmt.Printf("ss:%+v\n", f.Pulls[i])
	}
}
