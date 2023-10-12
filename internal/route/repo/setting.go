package repo

import (
	"errors"
	"fmt"
	"git-server/internal/conf"
	"git-server/internal/context"
	"git-server/internal/form"
	"git-server/internal/type"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func SettingsProtectedBranchPost(c *context.Context, f form.ProtectedBranch) {
	branch := f.BranchName
	if !c.Repo.GitRepo.HasBranch(branch) {
		c.JSON(500, _type.FaildResult(errors.New("branch is not exist")))
		return
	}
	branches, err := getProtectedBranch(c.Repo.RepoLink)
	if err != nil {
		c.JSON(500, _type.FaildResult(err))
		return
	}
	if f.Protected {
		for i := 0; i < len(branches); i++ {
			if branches[i] == branch {
				c.JSON(500, _type.FaildResult(errors.New("The branch is protected")))
				return
			}
		}
		if err := updateProtectedBranch(c.Repo.RepoLink, f); err != nil {
			c.JSON(500, _type.FaildResult(err))
		}
	} else {
		found := false
		for i := 0; i < len(branches); i++ {
			if branches[i] == branch {
				found = true
			}
		}
		if !found {
			c.JSON(500, _type.FaildResult(errors.New("The branch no protected")))
			return
		} else {
			if err := updateProtectedBranch(c.Repo.RepoLink, f); err != nil {
				c.JSON(500, _type.FaildResult(err))
			}
		}
	}

}
func SettingsProtectedBranch(c *context.Context) {
	branches, err := getProtectedBranch(c.Repo.RepoLink)
	if err != nil {
		c.JSON(500, _type.FaildResult(err))
	}
	c.JSON(200, _type.SuccessResult(branches))
}

func getProtectedBranch(repoLink string) ([]string, error) {
	repoPath := filepath.Join(conf.Repository.Root, repoLink) + ".git"
	filePath := filepath.Join(repoPath, "hooks", "pre-receive")
	// 读取文件内容
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, errors.New("File reading failure")
	}
	// 使用正则表达式匹配并替换
	re := regexp.MustCompile(`--branch='([^']*)'`)
	matches := re.FindStringSubmatch(string(content))
	if len(matches) != 2 {
		return nil, errors.New("No match found")
	}
	branches := strings.Split(matches[1], ",")
	return branches, nil
}

func updateProtectedBranch(repoLink string, f form.ProtectedBranch) error {
	repoPath := filepath.Join(conf.Repository.Root, repoLink) + ".git"
	filePath := filepath.Join(repoPath, "hooks", "pre-receive")
	// 打开文件
	file, err := os.OpenFile(filePath, os.O_RDWR, 0644)
	if err != nil {
		return errors.New("打开文件失败")
	}
	defer file.Close()

	// 读取文件内容
	content, err := ioutil.ReadAll(file)
	if err != nil {
		return errors.New("读取文件失败")
	}

	// 使用正则表达式匹配并替换
	re := regexp.MustCompile(`--branch='([^']*)'`)
	matches := re.FindStringSubmatch(string(content))
	if len(matches) != 2 {
		return errors.New("No match found")
	}
	oldValue := matches[1] // 旧值
	var newValue string    // 新值
	if f.Protected {
		newValue = oldValue + "," + f.BranchName
	} else {
		newValue = strings.ReplaceAll(oldValue, f.BranchName, "")
		newValue = strings.ReplaceAll(newValue, ",,", ",")
	}
	newValue = strings.Trim(newValue, ",")
	newContent := re.ReplaceAllString(string(content), fmt.Sprintf(`--branch='%s'`, newValue))

	// 将文件指针移至文件开始位置
	_, err = file.Seek(0, 0)
	if err != nil {
		return errors.New("重设文件指针失败")
	}

	// 清空文件内容
	err = file.Truncate(0)
	if err != nil {
		fmt.Printf("清空文件内容失败：%v\n", err)
		return errors.New("清空文件内容失败")
	}

	// 将修改后的内容写入文件
	_, err = file.Write([]byte(newContent))
	if err != nil {
		fmt.Printf("写入文件失败：%v\n", err)
		return errors.New("写入文件失败")
	}

	return nil
}
