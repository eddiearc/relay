package release

import (
	"fmt"
	"strings"
)

type Action string

const (
	ActionNoop               Action = "noop"
	ActionPublishExplicitTag Action = "publish-explicit-tag"
	ActionAutoCutPatch       Action = "auto-cut-patch"
)

type PolicyInput struct {
	LatestPublishedTag   string
	LatestPublishedSHA   string
	HeadSHA              string
	UnreleasedCommits    int
	CoveringExplicitTags []string
}

type Decision struct {
	Action               Action   `json:"action"`
	Tag                  string   `json:"tag,omitempty"`
	Reason               string   `json:"reason"`
	LatestPublishedTag   string   `json:"latest_published_tag,omitempty"`
	LatestPublishedSHA   string   `json:"latest_published_sha,omitempty"`
	HeadSHA              string   `json:"head_sha"`
	UnreleasedCommits    int      `json:"unreleased_commits"`
	CoveringExplicitTags []string `json:"covering_explicit_tags,omitempty"`
}

func Evaluate(input PolicyInput) (Decision, error) {
	decision := Decision{
		LatestPublishedTag:   input.LatestPublishedTag,
		LatestPublishedSHA:   input.LatestPublishedSHA,
		HeadSHA:              input.HeadSHA,
		UnreleasedCommits:    input.UnreleasedCommits,
		CoveringExplicitTags: append([]string(nil), input.CoveringExplicitTags...),
	}

	if strings.TrimSpace(input.HeadSHA) == "" {
		return Decision{}, fmt.Errorf("missing head commit")
	}
	if input.UnreleasedCommits < 0 {
		return Decision{}, fmt.Errorf("unreleased commit count must be non-negative")
	}

	if input.UnreleasedCommits == 0 || sameCommit(input.LatestPublishedSHA, input.HeadSHA) {
		decision.Action = ActionNoop
		decision.Reason = "latest published release already covers the current main commit"
		return decision, nil
	}

	if len(input.CoveringExplicitTags) > 0 {
		decision.Action = ActionPublishExplicitTag
		decision.Tag = highestStableTag(input.CoveringExplicitTags)
		decision.Reason = fmt.Sprintf("explicit release tag %s already covers the unreleased main commit set", decision.Tag)
		return decision, nil
	}

	if strings.TrimSpace(input.LatestPublishedTag) == "" {
		decision.Action = ActionNoop
		decision.Reason = "no published release baseline exists yet, so the first release must stay explicit"
		return decision, nil
	}

	nextPatch, err := nextPatchTag(input.LatestPublishedTag)
	if err != nil {
		return Decision{}, err
	}
	decision.Action = ActionAutoCutPatch
	decision.Tag = nextPatch
	decision.Reason = fmt.Sprintf("main has unreleased commits after %s and no explicit release tag covers HEAD", input.LatestPublishedTag)
	return decision, nil
}

func sameCommit(left, right string) bool {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	return left != "" && right != "" && left == right
}
