package cmd

import (
	"os"
	"path"
	"repo/internal/gitclient"
	"repo/internal/hoster"
	"repo/internal/hoster/gitlab"
	"repo/internal/say"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/logrusorgru/aurora"
	"github.com/spf13/cobra"
)

var cloneTopics []string
var cloneExcludePatterns []string
var cloneIncludePatterns []string

var cloneParallelism int
var cloneStarred bool

func init() {
	rootCmd.AddCommand(cloneCmd)
	cloneCmd.Flags().StringSliceVarP(&cloneTopics, "topic", "t", nil, "Topics (aka tags/labels) to be filtered. Multiple topics are possible (and).")
	cloneCmd.Flags().StringSliceVarP(&cloneExcludePatterns, "exclude", "e", nil, "Regex-pattern not to be matched for the path. Multiple patterns are possible (and).")
	cloneCmd.Flags().StringSliceVarP(&cloneIncludePatterns, "include", "i", nil, "Regex-pattern that needs to be matched for the path. Multiple patterns are possible (and).")
	cloneCmd.Flags().IntVarP(&cloneParallelism, "parallelism", "p", 64, "How many process should run in parallel, 1 would be no parallelism.")
	cloneCmd.Flags().BoolVarP(&cloneStarred, "starred", "s", false, "Filter for starred projects")
}

var cloneCmd = &cobra.Command{
	Use:   "clone [root-dir]",
	Short: "Clones selected repositories to the passed location. Adds new ones on reoccurring calls.",
	Long:  `Clones selected repositories to the passed location. Adds new ones on reoccurring calls.`,
	Args:  validateConditions(cobra.ExactArgs(1), validateArgGitDir(0, false, true)),
	Run: func(cmd *cobra.Command, args []string) {
		defer func(start time.Time) {
			say.InfoLn("%s Finished, took %s", aurora.Red("ॐ").String(), time.Since(start))
		}(time.Now())
		dirReposRoot := args[0]

		provider, err := gitlab.MakeProvider()
		handleFatalError(err)

		gitclient.PrepareSsh(provider.Host()) // TODO parse url?
		repos := provider.Repositories(hoster.RequestOptions{
			Topics:          cloneTopics,
			Starred:         cloneStarred,
			ExcludePatterns: cloneExcludePatterns,
			IncludePatterns: cloneIncludePatterns,
		})
		sort.Slice(repos, func(i, j int) bool {
			return repos[i].Name < repos[j].Name
		})

		repos = filterExisting(dirReposRoot, repos)
		cloneAll(dirReposRoot, repos)
	},
}

func filterExisting(dirReposRoot string, repos []hoster.ProviderRepository) (result []hoster.ProviderRepository) {
	for _, r := range repos {
		dirName := determineDirectoryName(r.SshUrl)
		dirRepository := path.Join(dirReposRoot, dirName)

		_, err := os.Stat(dirRepository)
		if os.IsNotExist(err) {
			result = append(result, r)
		} else {
			say.Verbose("Repository exists already: %s ", dirName)
		}
	}
	return
}

func determineDirectoryName(sshUrl string) string {
	last := sshUrl[strings.LastIndex(sshUrl, "/")+1:]
	if strings.HasSuffix(last, ".git") {
		last = last[:len(last)-4]
	}
	return last
}

func cloneAll(dirReposRoot string, repos []hoster.ProviderRepository) {
	tasks := make(chan string)
	var wg sync.WaitGroup
	counter := int32(0)
	for i := 0; i < getParallelism(cloneParallelism); i++ {
		wg.Add(1)
		go clone(dirReposRoot, &counter, len(repos), tasks, &wg)
	}

	for _, repo := range repos {
		tasks <- repo.SshUrl
	}

	close(tasks)
	wg.Wait()
}

func clone(dirReposRoot string, counter *int32, total int, tasks chan string, wg *sync.WaitGroup) {
	defer wg.Done()
	for sshUrl := range tasks {
		dirName := determineDirectoryName(sshUrl)
		err := gitclient.Clone(dirReposRoot, dirName, sshUrl)
		if err != nil {
			say.ProgressError(counter, total, err, dirName, "- Unable to clone")
		} else {
			say.ProgressSuccess(counter, total, dirName, "")
		}
	}
}
