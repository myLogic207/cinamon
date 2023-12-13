package ui

import (
	"fmt"
	"strings"

	"golang.org/x/crypto/ssh"
)

const (
	// length of the banner
	bannerLength  = 79
	bannerChar    = "-"
	bannerBorder  = "|"
	bannerSpacing = 4
)

const (
	// Left, Center, Right orientation
	orientationLeft = iota
	orientationCenter
	orientationRight
)

func Banner(conn ssh.ConnMetadata) string {
	builder := &strings.Builder{}
	length := 79
	endLine := fmt.Sprintf(".%s.\n", strings.Repeat(bannerChar, length-2))
	builder.WriteString(endLine)
	builder.WriteString(formatLine(fmt.Sprintf("Hello %s!", conn.User()), length, orientationLeft, bannerSpacing, true))
	builder.WriteString(formatLine("Welcome to patchssh!", length, orientationLeft, bannerSpacing, true))
	builder.WriteString(formatLine("", length, orientationCenter, 0, true))
	builder.WriteString(formatLine("!This is a test banner!", length, orientationCenter, bannerSpacing, true))
	builder.WriteString(endLine)
	return builder.String()
}

// fills the string up to the given length with spaces
// orientation: 0 = left, 1 = center, 2 = right
// spacing: number of spaces between text and border, aka opposite of orientation
func formatLine(raw string, length int, orientation int, spacing int, border bool) string {
	builder := &strings.Builder{}
	if border {
		length -= 2
		builder.WriteString(bannerBorder)
	}
	if len(raw) > length {
		builder.WriteString(raw[:length-3])
		builder.WriteString("...")
	} else if orientation == orientationLeft {
		builder.WriteString(strings.Repeat(" ", spacing))
		builder.WriteString(raw)
		builder.WriteString(strings.Repeat(" ", length-len(raw)-spacing))
	} else if orientation == orientationCenter {
		// middle ignores spacing
		left := (length - len(raw)) / 2
		right := length - len(raw) - left
		builder.WriteString(strings.Repeat(" ", left))
		builder.WriteString(raw)
		builder.WriteString(strings.Repeat(" ", right))
	} else if orientation == orientationRight {
		builder.WriteString(strings.Repeat(" ", length-len(raw)-spacing))
		builder.WriteString(raw)
		builder.WriteString(strings.Repeat(" ", spacing))
	}
	if border {
		builder.WriteString("|")
	}
	builder.WriteString("\n")
	return builder.String()
}
