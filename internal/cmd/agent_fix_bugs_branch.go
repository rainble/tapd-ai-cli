package cmd

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/studyzy/tapd-ai-cli/internal/tapdurl"
)

const branchStrategyLinkedMR = "linked-mr"

var (
	gitLabMRURLRe  = regexp.MustCompile(`https?://[^\s<>"']*/merge_requests/([0-9]+)`)
	safeGitArgRe   = regexp.MustCompile(`^[A-Za-z0-9._/-]+$`)
	candidateURLRe = regexp.MustCompile(`https?://[^\s<>"']+`)
)

type bugFixBranchOptions struct {
	Remote       string
	BranchPrefix string
	Limit        int
}

type bugFixBranchContext struct {
	MRURL       string
	MRIID       string
	LocalBranch string
	Source      string
}

type linkedMR struct {
	URL    string
	IID    string
	Source string
}

const maxStoryParentDepth = 16

func prepareLinkedMRBranch(ctx context.Context, tapd bugFixTapdClient, runner commandRunner, repo string, bug bugFixBugDetail, opts bugFixBranchOptions) (*bugFixBranchContext, error) {
	mr, err := findLinkedMR(ctx, tapd, bug)
	if err != nil {
		return nil, err
	}
	if mr.IID == "" {
		return nil, fmt.Errorf("no GitLab MR link found in TAPD story, parent story, or bug")
	}
	if err := validateGitArg(opts.Remote, "MR remote"); err != nil {
		return nil, err
	}
	if err := validateGitArg(opts.BranchPrefix, "MR branch prefix"); err != nil {
		return nil, err
	}

	localBranch := opts.BranchPrefix + mr.IID
	fetch := runner.Run(ctx, commandRunConfig{
		Dir:     repo,
		Command: fmt.Sprintf("git fetch %s merge-requests/%s/head", opts.Remote, mr.IID),
		Limit:   opts.Limit,
	})
	if fetch.Err != nil || fetch.ExitCode != 0 {
		return nil, fmt.Errorf("fetch MR %s failed:\n%s", mr.IID, commandFailureDetail(fetch))
	}
	checkout := runner.Run(ctx, commandRunConfig{
		Dir:     repo,
		Command: fmt.Sprintf("git checkout -B %s FETCH_HEAD", localBranch),
		Limit:   opts.Limit,
	})
	if checkout.Err != nil || checkout.ExitCode != 0 {
		return nil, fmt.Errorf("checkout MR %s failed:\n%s", mr.IID, commandFailureDetail(checkout))
	}
	return &bugFixBranchContext{
		MRURL:       mr.URL,
		MRIID:       mr.IID,
		LocalBranch: localBranch,
		Source:      mr.Source,
	}, nil
}

func findLinkedMR(ctx context.Context, tapd bugFixTapdClient, bug bugFixBugDetail) (linkedMR, error) {
	for _, storyID := range storyIDsForBug(bug) {
		mr, err := findMRInStoryParents(ctx, tapd, bug.WorkspaceID, storyID)
		if err != nil {
			return linkedMR{}, err
		}
		if mr.IID != "" {
			return mr, nil
		}
	}
	if mr := firstMRInTexts("bug:"+bug.ID, bug.Description, commentsText(bug.Comments)); mr.IID != "" {
		return mr, nil
	}
	return linkedMR{}, nil
}

func findMRInStoryParents(ctx context.Context, tapd bugFixTapdClient, workspaceID, storyID string) (linkedMR, error) {
	seen := map[string]bool{}
	for depth := 0; storyID != ""; depth++ {
		if depth >= maxStoryParentDepth {
			return linkedMR{}, fmt.Errorf("linked TAPD story parent chain exceeded %d levels at %s", maxStoryParentDepth, storyID)
		}
		if seen[storyID] {
			return linkedMR{}, fmt.Errorf("linked TAPD story parent chain has a cycle at %s", storyID)
		}
		seen[storyID] = true

		story, err := tapd.GetStoryDetail(ctx, workspaceID, storyID)
		if err != nil {
			return linkedMR{}, fmt.Errorf("load linked TAPD story %s failed: %w", storyID, err)
		}
		if mr := firstMRInTexts("story:"+storyID, story.Description, commentsText(story.Comments)); mr.IID != "" {
			return mr, nil
		}
		storyID = story.ParentID
	}
	return linkedMR{}, nil
}

func firstMRInTexts(source string, texts ...string) linkedMR {
	for _, text := range texts {
		text = strings.ReplaceAll(text, `\_`, "_")
		matches := gitLabMRURLRe.FindAllStringSubmatch(text, -1)
		for _, m := range matches {
			if len(m) >= 2 {
				return linkedMR{URL: m[0], IID: m[1], Source: source}
			}
		}
	}
	return linkedMR{}
}

func storyIDsForBug(bug bugFixBugDetail) []string {
	var ids []string
	seen := map[string]bool{}
	add := func(id string) {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		ids = append(ids, id)
	}
	add(bug.StoryID)
	for _, text := range append([]string{bug.Description}, commentsText(bug.Comments)) {
		for _, rawURL := range candidateURLRe.FindAllString(text, -1) {
			rawURL = strings.TrimRight(rawURL, ".,;)")
			parsed, err := tapdurl.Parse(rawURL)
			if err != nil || parsed.EntityType != "story" {
				continue
			}
			if bug.WorkspaceID == "" || parsed.WorkspaceID == bug.WorkspaceID {
				add(parsed.EntityID)
			}
		}
	}
	return ids
}

func commentsText(comments []bugFixComment) string {
	var b strings.Builder
	for _, c := range comments {
		if c.Description == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(c.Description)
	}
	return b.String()
}

func validateGitArg(v, label string) error {
	if v == "" {
		return fmt.Errorf("%s is required", label)
	}
	if !safeGitArgRe.MatchString(v) {
		return fmt.Errorf("%s contains unsupported characters: %q", label, v)
	}
	return nil
}
