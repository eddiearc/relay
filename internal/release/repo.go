package release

import (
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
)

type InspectOptions struct {
	MainRef              string
	PublishedReleaseTags []string
}

func InspectRepository(repoPath string, opts InspectOptions) (PolicyInput, error) {
	mainRef := strings.TrimSpace(opts.MainRef)
	if mainRef == "" {
		mainRef = "HEAD"
	}

	headSHA, err := gitOutput(repoPath, "rev-parse", mainRef)
	if err != nil {
		return PolicyInput{}, fmt.Errorf("resolve %s: %w", mainRef, err)
	}

	latestPublishedTag, latestPublishedSHA, err := latestPublishedTag(repoPath, headSHA, opts.PublishedReleaseTags)
	if err != nil {
		return PolicyInput{}, err
	}

	unreleasedCommits, err := countUnreleasedCommits(repoPath, latestPublishedTag, headSHA)
	if err != nil {
		return PolicyInput{}, err
	}

	coveringTags, err := coveringExplicitTags(repoPath, headSHA, opts.PublishedReleaseTags)
	if err != nil {
		return PolicyInput{}, err
	}

	return PolicyInput{
		LatestPublishedTag:   latestPublishedTag,
		LatestPublishedSHA:   latestPublishedSHA,
		HeadSHA:              headSHA,
		UnreleasedCommits:    unreleasedCommits,
		CoveringExplicitTags: coveringTags,
	}, nil
}

func latestPublishedTag(repoPath, headSHA string, publishedTags []string) (string, string, error) {
	var candidates []string
	for _, tag := range publishedTags {
		tag = strings.TrimSpace(tag)
		if !isStableReleaseTag(tag) {
			continue
		}
		if _, err := gitOutput(repoPath, "rev-parse", tag); err != nil {
			continue
		}
		if err := gitRun(repoPath, "merge-base", "--is-ancestor", tag, headSHA); err != nil {
			continue
		}
		candidates = append(candidates, tag)
	}
	if len(candidates) == 0 {
		return "", "", nil
	}
	sortStableTagsDescending(candidates)
	tag := candidates[0]
	sha, err := gitOutput(repoPath, "rev-parse", tag)
	if err != nil {
		return "", "", fmt.Errorf("resolve latest published tag %s: %w", tag, err)
	}
	return tag, sha, nil
}

func countUnreleasedCommits(repoPath, latestPublishedTag, headSHA string) (int, error) {
	args := []string{"rev-list", "--count"}
	if latestPublishedTag == "" {
		args = append(args, headSHA)
	} else {
		args = append(args, fmt.Sprintf("%s..%s", latestPublishedTag, headSHA))
	}
	out, err := gitOutput(repoPath, args...)
	if err != nil {
		return 0, fmt.Errorf("count unreleased commits: %w", err)
	}
	count, convErr := strconv.Atoi(strings.TrimSpace(out))
	if convErr != nil {
		return 0, fmt.Errorf("parse unreleased commit count %q: %w", out, convErr)
	}
	return count, nil
}

func coveringExplicitTags(repoPath, headSHA string, publishedTags []string) ([]string, error) {
	out, err := gitOutput(repoPath, "tag", "--points-at", headSHA)
	if err != nil {
		return nil, fmt.Errorf("list head tags: %w", err)
	}
	published := make(map[string]struct{}, len(publishedTags))
	for _, tag := range publishedTags {
		published[strings.TrimSpace(tag)] = struct{}{}
	}
	var tags []string
	for _, tag := range strings.Split(out, "\n") {
		tag = strings.TrimSpace(tag)
		if !isStableReleaseTag(tag) {
			continue
		}
		if _, ok := published[tag]; ok {
			continue
		}
		tags = append(tags, tag)
	}
	sortStableTagsDescending(tags)
	return tags, nil
}

func gitOutput(repoPath string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", repoPath}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(out))
		if message == "" {
			return "", err
		}
		return "", fmt.Errorf("%w: %s", err, message)
	}
	return strings.TrimSpace(string(out)), nil
}

func gitRun(repoPath string, args ...string) error {
	cmd := exec.Command("git", append([]string{"-C", repoPath}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(out))
		if message == "" {
			return err
		}
		return fmt.Errorf("%w: %s", err, message)
	}
	return nil
}

func nextPatchTag(tag string) (string, error) {
	if strings.TrimSpace(tag) == "" {
		return "v0.0.1", nil
	}
	parsed, ok := parseStableTag(tag)
	if !ok {
		return "", fmt.Errorf("latest published release tag %q is not a stable vX.Y.Z tag", tag)
	}
	parsed.patch += 1
	return parsed.String(), nil
}

type stableTag struct {
	major int
	minor int
	patch int
}

func (t stableTag) String() string {
	return fmt.Sprintf("v%d.%d.%d", t.major, t.minor, t.patch)
}

func isStableReleaseTag(tag string) bool {
	_, ok := parseStableTag(tag)
	return ok
}

func highestStableTag(tags []string) string {
	if len(tags) == 0 {
		return ""
	}
	copyTags := append([]string(nil), tags...)
	sortStableTagsDescending(copyTags)
	return copyTags[0]
}

func sortStableTagsDescending(tags []string) {
	sort.Slice(tags, func(i, j int) bool {
		left, leftOK := parseStableTag(tags[i])
		right, rightOK := parseStableTag(tags[j])
		if !leftOK || !rightOK {
			return tags[i] > tags[j]
		}
		if left.major != right.major {
			return left.major > right.major
		}
		if left.minor != right.minor {
			return left.minor > right.minor
		}
		return left.patch > right.patch
	})
}

func parseStableTag(tag string) (stableTag, bool) {
	var parsed stableTag
	trimmed := strings.TrimSpace(tag)
	if _, err := fmt.Sscanf(trimmed, "v%d.%d.%d", &parsed.major, &parsed.minor, &parsed.patch); err != nil {
		return stableTag{}, false
	}
	if parsed.String() != trimmed {
		return stableTag{}, false
	}
	return parsed, true
}
