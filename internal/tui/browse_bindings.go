package tui

import (
	openbindings "github.com/openbindings/openbindings-go"
)

func bindingDisplayName(entry *openbindings.BindingEntry, iface *openbindings.Interface) string {
	if entry == nil || iface == nil {
		return ""
	}
	sourceName := entry.Source
	if src, ok := iface.Sources[entry.Source]; ok && src.Format != "" {
		sourceName = src.Format
	}
	return sourceName
}
