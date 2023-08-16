package _type

import "github.com/gogs/git-module"

type EntryCommitInfo struct {
	Entry     map[string]interface{}
	Index     int
	Commit    *git.Commit
	Submodule *git.Submodule
}

type ResultType struct {
	Code    int64  `json:"code"`
	Data    any    `json:"data"`
	Msg     string `json:"msg"`
	Success bool   `json:"success"`
}

func ProduceResult(data []*git.EntryCommitInfo) []EntryCommitInfo {
	entryCommitInfos := make([]EntryCommitInfo, len(data))
	for i := 0; i < len(data); i++ {
		m := make(map[string]interface{})
		m["Mode"] = data[i].Entry.Mode()
		m["Id"] = data[i].Entry.ID()
		m["Name"] = data[i].Entry.Name()
		m["Size"] = data[i].Entry.Size()
		m["Typ"] = data[i].Entry.Type()
		entryCommitInfos[i].Entry = m
		entryCommitInfos[i].Index = data[i].Index
		entryCommitInfos[i].Commit = data[i].Commit
		entryCommitInfos[i].Submodule = data[i].Submodule
	}
	return entryCommitInfos
}
