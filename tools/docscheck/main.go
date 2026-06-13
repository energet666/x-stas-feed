package main

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	adrFilenamePattern  = regexp.MustCompile(`^(\d{4})-[a-z0-9]+(?:-[a-z0-9]+)*\.md$`)
	markdownLinkPattern = regexp.MustCompile(`!?\[[^\]]*\]\(([^)]+)\)`)
	allowedTypes        = map[string]bool{
		"index":     true,
		"reference": true,
		"decision":  true,
		"template":  true,
	}
)

type note struct {
	path  string
	meta  map[string]string
	body  string
	isADR bool
}

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}

	problems := Check(root)
	if len(problems) == 0 {
		fmt.Println("documentation check passed")
		return
	}
	for _, problem := range problems {
		fmt.Fprintln(os.Stderr, problem)
	}
	os.Exit(1)
}

// Check validates the repository documentation rooted at root.
func Check(root string) []error {
	var problems []error
	docsRoot := filepath.Join(root, "docs")
	notes, readProblems := readNotes(docsRoot)
	problems = append(problems, readProblems...)

	decisions := make(map[int]string)
	for _, current := range notes {
		problems = append(problems, validateNote(root, current, decisions)...)
		problems = append(problems, validateLinks(root, current.path, current.body)...)
	}

	for _, relativePath := range []string{"AGENTS.md", "README.md"} {
		path := filepath.Join(root, relativePath)
		data, err := os.ReadFile(path)
		if err != nil {
			problems = append(problems, fmt.Errorf("%s: %w", relativePath, err))
			continue
		}
		content := string(data)
		problems = append(problems, validateLegacyReferences(relativePath, content)...)
		problems = append(problems, validateLinks(root, relativePath, content)...)
	}
	for _, current := range notes {
		problems = append(problems, validateLegacyReferences(current.path, current.body)...)
	}

	sort.Slice(problems, func(i, j int) bool {
		return problems[i].Error() < problems[j].Error()
	})
	return problems
}

func readNotes(docsRoot string) ([]note, []error) {
	var notes []note
	var problems []error
	err := filepath.WalkDir(docsRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			problems = append(problems, walkErr)
			return nil
		}
		if entry.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			problems = append(problems, err)
			return nil
		}
		relativePath, err := filepath.Rel(docsRoot, path)
		if err != nil {
			problems = append(problems, err)
			return nil
		}
		meta, body, err := parseFrontmatter(string(data))
		if err != nil {
			problems = append(problems, fmt.Errorf("docs/%s: %w", filepath.ToSlash(relativePath), err))
		}
		notes = append(notes, note{
			path: "docs/" + filepath.ToSlash(relativePath),
			meta: meta,
			body: body,
			isADR: filepath.ToSlash(filepath.Dir(relativePath)) == "decisions" &&
				adrFilenamePattern.MatchString(filepath.Base(relativePath)),
		})
		return nil
	})
	if err != nil {
		problems = append(problems, err)
	}
	if len(notes) == 0 {
		problems = append(problems, errors.New("docs: no Markdown notes found"))
	}
	return notes, problems
}

func parseFrontmatter(content string) (map[string]string, string, error) {
	meta := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(content))
	if !scanner.Scan() || strings.TrimSpace(scanner.Text()) != "---" {
		return meta, content, errors.New("missing opening frontmatter delimiter")
	}

	var body []string
	closed := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "---" {
			closed = true
			break
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok || strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			return meta, content, fmt.Errorf("invalid frontmatter line %q", line)
		}
		key = strings.TrimSpace(key)
		if _, exists := meta[key]; exists {
			return meta, content, fmt.Errorf("duplicate frontmatter key %q", key)
		}
		meta[key] = strings.Trim(strings.TrimSpace(value), `"'`)
	}
	if err := scanner.Err(); err != nil {
		return meta, content, err
	}
	if !closed {
		return meta, content, errors.New("missing closing frontmatter delimiter")
	}
	for scanner.Scan() {
		body = append(body, scanner.Text())
	}
	return meta, strings.Join(body, "\n"), scanner.Err()
}

func validateNote(root string, current note, decisions map[int]string) []error {
	var problems []error
	for _, key := range []string{"title", "type", "status"} {
		if strings.TrimSpace(current.meta[key]) == "" {
			problems = append(problems, fmt.Errorf("%s: missing %s frontmatter", current.path, key))
		}
	}

	noteType := current.meta["type"]
	if noteType != "" && !allowedTypes[noteType] {
		problems = append(problems, fmt.Errorf("%s: unsupported type %q", current.path, noteType))
	}

	if current.isADR {
		if noteType != "decision" {
			problems = append(problems, fmt.Errorf("%s: numbered ADR must have type decision", current.path))
		}
		if strings.TrimSpace(current.meta["date"]) == "" {
			problems = append(problems, fmt.Errorf("%s: missing date frontmatter", current.path))
		}
		decision, err := strconv.Atoi(current.meta["decision"])
		if err != nil || decision <= 0 {
			problems = append(problems, fmt.Errorf("%s: decision must be a positive integer", current.path))
		} else {
			filenameMatch := adrFilenamePattern.FindStringSubmatch(filepath.Base(current.path))
			filenameDecision, _ := strconv.Atoi(filenameMatch[1])
			if decision != filenameDecision {
				problems = append(problems, fmt.Errorf(
					"%s: decision %d does not match filename number %04d",
					current.path, decision, filenameDecision,
				))
			}
			if previous, exists := decisions[decision]; exists {
				problems = append(problems, fmt.Errorf(
					"%s: decision %d duplicates %s", current.path, decision, previous,
				))
			} else {
				decisions[decision] = current.path
			}
		}
		if current.meta["status"] == "superseded" {
			target := strings.TrimSpace(current.meta["superseded-by"])
			if target == "" {
				problems = append(problems, fmt.Errorf(
					"%s: superseded ADR requires superseded-by", current.path,
				))
			} else {
				problems = append(
					problems,
					validateRelativeTarget(root, current.path, target, "superseded-by")...,
				)
			}
		}
	} else if noteType == "decision" {
		problems = append(problems, fmt.Errorf(
			"%s: decision notes must use NNNN-slug.md in docs/decisions", current.path,
		))
	}

	return problems
}

func validateLinks(root, sourcePath, content string) []error {
	var problems []error
	for _, match := range markdownLinkPattern.FindAllStringSubmatch(content, -1) {
		rawTarget := strings.TrimSpace(match[1])
		if rawTarget == "" || strings.HasPrefix(rawTarget, "#") {
			continue
		}
		if strings.Contains(rawTarget, "://") || strings.HasPrefix(rawTarget, "mailto:") {
			continue
		}
		target := strings.SplitN(rawTarget, "#", 2)[0]
		target = strings.SplitN(target, " ", 2)[0]
		decoded, err := url.PathUnescape(target)
		if err != nil {
			problems = append(problems, fmt.Errorf("%s: invalid link %q", sourcePath, rawTarget))
			continue
		}

		sourceOnDisk := filepath.Join(root, filepath.FromSlash(sourcePath))
		resolved := filepath.Clean(filepath.Join(filepath.Dir(sourceOnDisk), filepath.FromSlash(decoded)))
		if _, err := os.Stat(resolved); err != nil {
			problems = append(problems, fmt.Errorf("%s: broken link %q", sourcePath, rawTarget))
		}
	}
	return problems
}

func validateRelativeTarget(root, sourcePath, target, field string) []error {
	if strings.HasPrefix(target, "/") || strings.Contains(target, "://") {
		return []error{fmt.Errorf("%s: %s must be a relative Markdown link", sourcePath, field)}
	}
	target = strings.SplitN(target, "#", 2)[0]
	if filepath.Ext(target) != ".md" {
		return []error{fmt.Errorf("%s: %s must point to a Markdown ADR", sourcePath, field)}
	}
	resolved := filepath.Clean(filepath.Join(
		root,
		filepath.FromSlash(filepath.Dir(sourcePath)),
		filepath.FromSlash(target),
	))
	if _, err := os.Stat(resolved); err != nil {
		return []error{fmt.Errorf("%s: broken %s %q", sourcePath, field, target)}
	}
	return nil
}

func validateLegacyReferences(path, content string) []error {
	var problems []error
	for _, legacy := range []string{"MEMORY.md", "DESIGN.md"} {
		if strings.Contains(content, legacy) {
			problems = append(problems, fmt.Errorf("%s: references removed %s", path, legacy))
		}
	}
	return problems
}
