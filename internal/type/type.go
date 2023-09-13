package _type

import "github.com/gogs/git-module"

type EntryCommitInfo struct {
	Entry     map[string]interface{}
	Index     int
	Commit    map[string]interface{}
	Submodule *git.Submodule
}

type ResultType struct {
	Code    int64  `json:"code"`
	Data    any    `json:"data"`
	Msg     string `json:"msg"`
	Success bool   `json:"success"`
}
type LastCommit map[string]interface{}

func ProduceLastCommit(data *git.Commit) LastCommit {
	m := make(map[string]interface{})
	m["ID"] = data.ID.String()
	m["Author"] = data.Author
	m["Committer"] = data.Committer
	m["Message"] = data.Message
	return m
}

func ProduceEntryCommitInfo(data []*git.EntryCommitInfo) []EntryCommitInfo {
	entryCommitInfos := make([]EntryCommitInfo, len(data))
	for i := 0; i < len(data); i++ {
		m := make(map[string]interface{})
		m["Mode"] = data[i].Entry.Mode()
		m["Id"] = data[i].Entry.ID()
		m["Name"] = data[i].Entry.Name()
		m["Size"] = data[i].Entry.Size()
		m["Typ"] = data[i].Entry.Type()
		entryCommitInfos[i].Entry = m
		m2 := make(map[string]interface{})
		m2["ID"] = data[i].Commit.ID.String()
		m2["Author"] = data[i].Commit.Author
		m2["Committer"] = data[i].Commit.Committer
		m2["Message"] = data[i].Commit.Message
		entryCommitInfos[i].Commit = m2
		entryCommitInfos[i].Index = data[i].Index
		entryCommitInfos[i].Submodule = data[i].Submodule
	}
	return entryCommitInfos
}
