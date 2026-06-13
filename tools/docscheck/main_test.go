package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestCheckValidDocumentation(t *testing.T) {
	root := newFixture(t)
	writeNote(t, root, "docs/index.md", `---
title: Index
type: index
status: active
---
# Index

[Decision](decisions/0001-test-decision.md)
`)
	writeNote(t, root, "docs/decisions/0001-test-decision.md", `---
title: Test Decision
type: decision
status: accepted
date: 2026-06-14
decision: 1
---
# Test
`)

	if problems := Check(root); len(problems) != 0 {
		t.Fatalf("Check() problems = %v", problems)
	}
}

func TestCheckReportsBrokenLink(t *testing.T) {
	root := newFixture(t)
	writeNote(t, root, "docs/index.md", `---
title: Index
type: index
status: active
---
[Missing](missing.md)
`)

	assertProblemContains(t, Check(root), "broken link")
}

func TestCheckReportsMalformedFrontmatter(t *testing.T) {
	root := newFixture(t)
	writeNote(t, root, "docs/index.md", "# No frontmatter\n")

	assertProblemContains(t, Check(root), "missing opening frontmatter delimiter")
}

func TestCheckReportsDuplicateDecisionID(t *testing.T) {
	root := newFixture(t)
	writeNote(t, root, "docs/index.md", validIndex)
	writeNote(t, root, "docs/decisions/0001-first.md", decisionNote(1, "accepted", ""))
	writeNote(t, root, "docs/decisions/0002-second.md", decisionNote(1, "accepted", ""))

	assertProblemContains(t, Check(root), "duplicates")
}

func TestCheckRequiresSupersededReplacement(t *testing.T) {
	root := newFixture(t)
	writeNote(t, root, "docs/index.md", validIndex)
	writeNote(t, root, "docs/decisions/0001-old.md", decisionNote(1, "superseded", ""))

	assertProblemContains(t, Check(root), "requires superseded-by")
}

func TestCheckAcceptsSupersededReplacement(t *testing.T) {
	root := newFixture(t)
	writeNote(t, root, "docs/index.md", validIndex)
	writeNote(t, root, "docs/decisions/0001-old.md", decisionNote(
		1, "superseded", "superseded-by: 0002-new.md\n",
	))
	writeNote(t, root, "docs/decisions/0002-new.md", decisionNote(2, "accepted", ""))

	if problems := Check(root); len(problems) != 0 {
		t.Fatalf("Check() problems = %v", problems)
	}
}

const validIndex = `---
title: Index
type: index
status: active
---
# Index
`

func decisionNote(id int, status, extra string) string {
	return `---
title: Decision
type: decision
status: ` + status + `
date: 2026-06-14
decision: ` + strconv.Itoa(id) + `
` + extra + `---
# Decision
`
}

func newFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeNote(t, root, "AGENTS.md", "# Agents\n")
	writeNote(t, root, "README.md", "# Readme\n")
	return root
}

func writeNote(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertProblemContains(t *testing.T, problems []error, text string) {
	t.Helper()
	for _, problem := range problems {
		if strings.Contains(problem.Error(), text) {
			return
		}
	}
	t.Fatalf("problems %v do not contain %q", problems, text)
}
