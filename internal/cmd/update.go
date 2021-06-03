package cmd

import (
	"errors"
	"fmt"
	"repo/internal/gitclient"
	"repo/internal/hoster/gitlab"
	"repo/internal/model"
	"repo/internal/say"
	"repo/internal/util"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/logrusorgru/aurora"
	"github.com/spf13/cobra"
)

var updateQuiet bool
var updateParallel bool

func init() {
	rootCmd.AddCommand(updateCmd)
	updateCmd.Flags().BoolVarP(&updateQuiet, "quiet", "q", false, "Output only affected repositories")
	updateCmd.Flags().BoolVarP(&updateParallel, "parallel", "p", true, "Process operations in parallel")
}

var updateCmd = &cobra.Command{
	Use:   "update [mode] [dir]",
	Short: "Checks/fetches/pulls updates for the given repository/repositories",
	Long: `Checks/fetches/pulls updates for the given repository or repositories below the given directory.

Mode can be one of:
  check - Outputs the current state of the repositories
  fetch - Fetches remote changes and outputs the changes
  pull  - Fetches remote changes, merges them (if fast-forward is possible) and outputs the changes`,
	Args: validateConditions(cobra.ExactArgs(2), validateArgGitDir(1, false, true)),
	Run: func(cmd *cobra.Command, args []string) {
		defer func(start time.Time) { say.InfoLn("🦊 Finished, took %s", time.Since(start)) }(time.Now())
		modes := []string{"check", "fetch", "pull"}

		mode := args[0]
		provider, err := gitlab.MakeProvider()
		handleFatalError(err)

		if !util.IsInSlice(mode, modes...) {
			handleFatalError(errors.New(fmt.Sprintf("mode has to be one of: %s", modes)))
		}

		gitDirs := collectGitDirsHandled(args[1], provider)

		if mode == "fetch" || mode == "pull" {
			gitclient.PrepareSsh(provider.Host())
		}

		counter := int32(0)

		// channel to send new jobs
		ch := make(chan *StateContext)

		var wg sync.WaitGroup

		// create 6 worker routines
		for i := 0; i < 6; i++ {
			wg.Add(1)
			go processRepository(mode, ch, &wg, "foo")
		}

		for _, gd := range gitDirs {
			var rdIntermediate model.RepoDir
			rdIntermediate = gd
			sc := &StateContext{total: len(gitDirs), counter: &counter, repo: &rdIntermediate}
			// send an operation
			ch <- sc

			// if updateParallel {
			// 	go processRepository(mode, ch, &wg, sc.repo.PathDirName())
			// } else {
			// 	processRepository(mode, ch, &wg, sc.repo.PathDirName())
			// }
		}

		// closing the channel will end the worker routines (for loop stops)
		close(ch)
		// wait for the worker are signal Done in wg
		wg.Wait()
	},
}

type State int

const (
	clean State = iota
	dirty
	failed
)

type StateContext struct {
	total   int
	counter *int32
	mutex   sync.Mutex // avoid mixed outputs
	repo    *model.RepoDir
	state   State
	ref     string
	behind  int
	message string
}

func processRepository(mode string, ch chan *StateContext, wg *sync.WaitGroup, foo string) {
	// wait for messages in channel
	for ctx := range ch {
		if false {
			time.Sleep(1 * time.Second)
			//say.InfoLn("%s %s", ctx.repo.Name, foo)
			return
		}

		ctx.ref = gitclient.GetCurrentBranch(ctx.repo.Path)
		switch mode {
		case "check":
			updateCheck(ctx)
		case "fetch":
			updateFetch(ctx)
		case "pull":
			updateFetch(ctx)
			if ctx.state != failed && ctx.state != clean {
				updatePull(ctx)
			}
		}
		printContext(ctx)
	}
	// signal waitgroup that we are done
	wg.Done()
}

func updateCheck(ctx *StateContext) {
	if gitclient.IsDirty(ctx.repo.Path) {
		ctx.message = gitclient.GetLocalChanges(ctx.repo.Path)
		ctx.state = dirty
	} else {
		ctx.state = clean
	}
}

func updateFetch(ctx *StateContext) {
	fetched := gitclient.Fetch(ctx.repo.Path)
	if !fetched {
		ctx.state = failed
		ctx.message = "Could not be fetched"
		return
	}
	if gitclient.IsEmpty(ctx.repo.Path) {
		ctx.state = failed
		ctx.message = "Empty git repository"
		return
	}
	ctx.behind = gitclient.GetBehindCount(ctx.repo.Path, ctx.ref)
	if ctx.behind == 0 {
		ctx.state = clean
		return
	}
	remotes := gitclient.IsRemoteExisting(ctx.repo.Path, ctx.ref)
	if !remotes {
		ctx.state = clean
		ctx.message = "No remote for the current branch"
		return
	}
	ctx.state = dirty
	ctx.message = gitclient.GetChanges(ctx.repo.Path, ctx.behind)
	return
}

func updatePull(ctx *StateContext) {
	success := gitclient.MergeFF(ctx.repo.Path)
	if !success {
		ctx.message = "Can not be merged, conflicting changes"
		ctx.state = failed
		return
	}
}

func printContext(ctx *StateContext) {
	ctx.mutex.Lock()
	var outState string
	switch ctx.state {
	case clean:
		if updateQuiet {
			return
		}
		outState = aurora.Green("✔").Bold().String()
	case dirty:
		outState = aurora.Yellow("●").Bold().String()
	case failed:
		outState = aurora.Red("✖").Bold().String()
	default:
		outState = "?"
	}

	// 80 chars for the separator, minus name of repo, minus spaces
	outSep := strings.Repeat("_", 80-len(ctx.repo.Name)-1)
	outBranch := aurora.Magenta(ctx.ref).String()
	outBehind := ""
	if ctx.behind > 0 {
		outBehind = "↓" + strconv.Itoa(ctx.behind)
	}
	say.ProgressGeneric(ctx.counter, ctx.total, outState, ctx.repo.Name, "%s (%s%s)", outSep, outBranch, outBehind)

	msg := strings.TrimSpace(ctx.message)
	if len(msg) > 0 {
		if ctx.state == failed {
			say.Raw(aurora.Red(msg).String() + "\n")
		} else {
			say.Raw(msg + "\n")
		}
	}
	ctx.mutex.Unlock()
}
