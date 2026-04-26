package object

import (
	"bytes"
	"fmt"
	"strings"
)

// Commit represents a Git commit object.
type Commit struct {
	Tree      string   // hex hash of root tree
	Parents   []string // hex hashes of parent commits (0, 1, or more)
	Author    string   // full author line: "Name <email> timestamp tz"
	Committer string   // full committer line
	Message   string   // commit message
}

func (c *Commit) Type() Type { return TypeCommit }

// Serialize encodes the commit in Git's text format.
func (c *Commit) Serialize() ([]byte, error) {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "tree %s\n", c.Tree)
	for _, p := range c.Parents {
		fmt.Fprintf(&buf, "parent %s\n", p)
	}
	fmt.Fprintf(&buf, "author %s\n", c.Author)
	fmt.Fprintf(&buf, "committer %s\n", c.Committer)
	buf.WriteByte('\n')
	buf.WriteString(c.Message)
	// Ensure message ends with newline
	if len(c.Message) > 0 && c.Message[len(c.Message)-1] != '\n' {
		buf.WriteByte('\n')
	}
	return buf.Bytes(), nil
}

// ParseCommit decodes commit content bytes into a Commit struct.
func ParseCommit(content []byte) (*Commit, error) {
	text := string(content)
	// Split header block and message at the first blank line
	idx := strings.Index(text, "\n\n")
	if idx < 0 {
		return nil, fmt.Errorf("malformed commit: no blank line separating header and message")
	}
	headerBlock := text[:idx]
	message := text[idx+2:]

	c := &Commit{Message: message}
	for _, line := range strings.Split(headerBlock, "\n") {
		if strings.HasPrefix(line, "tree ") {
			c.Tree = strings.TrimPrefix(line, "tree ")
		} else if strings.HasPrefix(line, "parent ") {
			c.Parents = append(c.Parents, strings.TrimPrefix(line, "parent "))
		} else if strings.HasPrefix(line, "author ") {
			c.Author = strings.TrimPrefix(line, "author ")
		} else if strings.HasPrefix(line, "committer ") {
			c.Committer = strings.TrimPrefix(line, "committer ")
		}
	}
	if c.Tree == "" {
		return nil, fmt.Errorf("malformed commit: missing tree field")
	}
	return c, nil
}
