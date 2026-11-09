package model

import "time"

// Signature is a person and a timestamp, used for both the author and the
// committer of a commit. Time is the instant from the Git timestamp; the
// zone is preserved so formatting can show the original offset.
type Signature struct {
	Name  string
	Email string
	When  time.Time
}

// Equal reports whether the name, email, and instant match.
// Time zones that represent the same instant are equal.
func (s Signature) Equal(other Signature) bool {
	return s.Name == other.Name && s.Email == other.Email && s.When.Equal(other.When)
}

// Commit is one commit's metadata: hash, parents, both signatures, the
// subject line, and the full message.
//
// Subject is the first line of the message (the part before the first
// blank line). Message is the raw message, including the subject.
// Parents is empty for a root commit and has more than one element for a
// merge. The order is the order Git records, first parent first.
type Commit struct {
	Hash      Hash
	Parents   []Hash
	Author    Signature
	Committer Signature
	Subject   string
	Message   string
}

// Equal compares hash, parent hashes (in order), both signatures, subject,
// and message.
func (c Commit) Equal(other Commit) bool {
	if !c.Hash.Equal(other.Hash) || c.Subject != other.Subject || c.Message != other.Message {
		return false
	}
	if !c.Author.Equal(other.Author) || !c.Committer.Equal(other.Committer) {
		return false
	}
	if len(c.Parents) != len(other.Parents) {
		return false
	}
	for i := range c.Parents {
		if !c.Parents[i].Equal(other.Parents[i]) {
			return false
		}
	}
	return true
}

// SubjectOf returns the subject line of a commit message: everything up to
// the first newline, with a trailing '\r' removed. The remainder of the
// message is not inspected; Git treats the first line as the subject even
// when a blank line does not follow.
func SubjectOf(message string) string {
	if i := indexNewline(message); i >= 0 {
		line := message[:i]
		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}
		return line
	}
	return message
}

func indexNewline(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			return i
		}
	}
	return -1
}
