package app

import (
	"strings"

	"github.com/openbindings/cli/internal/delegates"
)

// RenderSoftwareInfo returns a human-friendly styled representation of SoftwareInfo.
func RenderSoftwareInfo(sw delegates.SoftwareInfo) string {
	s := Styles
	var sb strings.Builder

	sb.WriteString(s.Header.Render(sw.Name))
	if sw.Version != "" {
		sb.WriteString(s.Dim.Render(" v" + sw.Version))
	}

	if sw.Description != "" {
		sb.WriteString("\n")
		sb.WriteString(sw.Description)
	}

	if sw.Homepage != "" || sw.Repository != "" || sw.Maintainer != "" {
		sb.WriteString("\n")
	}

	if sw.Maintainer != "" {
		sb.WriteString("\n  ")
		sb.WriteString(s.Bullet.Render("•"))
		sb.WriteString(" ")
		sb.WriteString(s.Dim.Render("Maintainer: "))
		sb.WriteString(sw.Maintainer)
	}

	if sw.Homepage != "" {
		sb.WriteString("\n  ")
		sb.WriteString(s.Bullet.Render("•"))
		sb.WriteString(" ")
		sb.WriteString(s.Dim.Render("Homepage:   "))
		sb.WriteString(s.Key.Render(sw.Homepage))
	}

	if sw.Repository != "" {
		sb.WriteString("\n  ")
		sb.WriteString(s.Bullet.Render("•"))
		sb.WriteString(" ")
		sb.WriteString(s.Dim.Render("Repository: "))
		sb.WriteString(s.Key.Render(sw.Repository))
	}

	return sb.String()
}

// Info returns ob's own software identity and metadata.
func Info() delegates.SoftwareInfo {
	return delegates.SoftwareInfo{
		Name:        "OpenBindings CLI",
		Version:     OBVersion,
		Description: "Reference implementation for creating, browsing, and executing OpenBindings interfaces.",
		Homepage:    "https://openbindings.com",
		Repository:  "https://github.com/openbindings/cli",
		Maintainer:  "OpenBindings Project",
	}
}
