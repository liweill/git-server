package doc

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/gogs/git-module"
	"github.com/unknwon/com"
	"github.com/urfave/cli"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

var (
	Hook = cli.Command{
		Name:        "hook",
		Usage:       "Delegate commands to corresponding Git hooks",
		Description: "All sub-commands should only be called by Git",
		Flags: []cli.Flag{
			stringFlag("branch, b", "", "Protected branch"),
		},
		Subcommands: []cli.Command{
			subcmdHookPreReceive,
		},
	}

	subcmdHookPreReceive = cli.Command{
		Name:        "pre-receive",
		Usage:       "Delegate pre-receive Git hook",
		Description: "This command should only be called by Git",
		Action:      runHookPreReceive,
	}
)

func runHookPreReceive(c *cli.Context) error {
	fmt.Println("runHookPreReceive runHookPreReceive")
	if os.Getenv("SSH_ORIGINAL_COMMAND") == "" {
		return nil
	}
	branchs := make([]string, 0)
	if c.GlobalIsSet("branch") {
		branchs = append(branchs, strings.Split(c.GlobalString("branch"), ",")...)
	}
	isWiki := strings.Contains(os.Getenv(ENV_REPO_CUSTOM_HOOKS_PATH), ".wiki.git/")
	for i := 0; i < len(branchs); i++ {
		fmt.Println("分支", branchs[i])
	}
	buf := bytes.NewBuffer(nil)
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		buf.Write(scanner.Bytes())
		buf.WriteByte('\n')

		if isWiki {
			continue
		}

		fields := bytes.Fields(scanner.Bytes())
		if len(fields) != 3 {
			continue
		}
		oldCommitID := string(fields[0])
		newCommitID := string(fields[1])
		branchName := git.RefShortName(string(fields[2]))

		if len(branchs) == 0 {
			continue
		}

		fmt.Printf("受保护的分支：%+v", "baohu")

		flag := false
		for i := 0; i < len(branchs); i++ {
			if branchs[i] == branchName {
				flag = true
			}
		}
		if flag {
			fail(fmt.Sprintf("Branch '%s' is protected and commits must be merged through pull request", branchName), "")
		} else {
			continue
		}

		// check and deletion
		if newCommitID == git.EmptyID {
			fmt.Println("11111111")
			fail(fmt.Sprintf("Branch '%s' is protected from deletion", branchName), "")
		}
		fail(fmt.Sprintf("Branch '%s' is protected from deletion", "master"), "")
		// Check force push
		output, err := git.NewCommand("rev-list", "--max-count=1", oldCommitID, "^"+newCommitID).
			RunInDir(RepoPath(os.Getenv(ENV_REPO_OWNER_NAME), os.Getenv(ENV_REPO_NAME)))
		if err != nil {
			fmt.Println("22222222222")
			fail("Internal error", "Failed to detect force push: %v", err)
		} else if len(output) > 0 {
			fmt.Println("3333333333")
			fail(fmt.Sprintf("Branch '%s' is protected from force push", branchName), "")
		}
	}

	customHooksPath := filepath.Join(os.Getenv(ENV_REPO_CUSTOM_HOOKS_PATH), "pre-receive")
	if !com.IsFile(customHooksPath) {
		return nil
	}

	var hookCmd *exec.Cmd
	if IsWindowsRuntime() {
		hookCmd = exec.Command("bash.exe", "custom_hooks/pre-receive")
	} else {
		hookCmd = exec.Command(customHooksPath)
	}
	hookCmd.Dir = RepoPath(os.Getenv(ENV_REPO_OWNER_NAME), os.Getenv(ENV_REPO_NAME))
	hookCmd.Stdout = os.Stdout
	hookCmd.Stdin = buf
	hookCmd.Stderr = os.Stderr
	if err := hookCmd.Run(); err != nil {
		fmt.Println("444444444444")
		fail("Internal error", "Failed to execute custom pre-receive hook: %v", err)
	}
	return nil
}
func fail(userMessage, errMessage string, args ...any) {
	_, _ = fmt.Fprintln(os.Stderr, "Gogs:", userMessage)

	if len(errMessage) > 0 {
		//if !conf.IsProdMode() {
		//	fmt.Fprintf(os.Stderr, errMessage+"\n", args...)
		//}
		fmt.Fprintf(os.Stderr, errMessage+"\n", args...)
		//log.Error(errMessage, args...)
	}

	os.Exit(1)
}
func RepoPath(userName, repoName string) string {
	return filepath.Join("C:/Users/15713/gogs-repositories", userName, repoName)
}

// IsWindowsRuntime returns true if the current runtime in Windows.
func IsWindowsRuntime() bool {
	return runtime.GOOS == "windows"
}
